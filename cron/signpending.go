package main

import (
	"context"
	"encoding/base64"
	"github.com/data-preservation-programs/filsigner-relayed/client"
	"github.com/data-preservation-programs/filsigner-relayed/config"
	filcrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"sync/atomic"
	"time"

	filaddr "github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	filmarket "github.com/filecoin-project/go-state-types/builtin/v9/market"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/ribasushi/go-toolbox/cmn"
	"github.com/ribasushi/go-toolbox/ufcli"
	"github.com/ribasushi/spade/internal/app"
)

type signTotals struct {
	signed  *int32
	timeout *int32
	failed  *int32
}

var signPending = &ufcli.Command{
	Usage: "Sign pending deal proposals",
	Name:  "sign-pending",
	Flags: []ufcli.Flag{},
	Action: func(cctx *ufcli.Context) error {
		ctx, log, db, gctx := app.UnpackCtx(cctx.Context)

		identityKeyBytes, err := base64.StdEncoding.DecodeString(gctx.PeerPrivateKey)
		if err != nil {
			return cmn.WrErr(errors.Wrap(err, "cannot decode identity private key"))
		}

		identityKey, err := crypto.UnmarshalPrivateKey(identityKeyBytes)
		if err != nil {
			return cmn.WrErr(errors.Wrap(err, "cannot unmarshal identity private key"))
		}

		filsigner, err := client.NewClient(identityKey, config.GetDefaultRelayInfo())
		if err != nil {
			return cmn.WrErr(errors.Wrap(err, "cannot create filsigner client"))
		}

		totals := signTotals{
			signed:  new(int32),
			failed:  new(int32),
			timeout: new(int32),
		}
		wallets := make(map[filaddr.Address]struct{}, 16)
		defer func() {
			log.Infow("summary",
				"uniqueWallets", len(wallets),
				"successful", atomic.LoadInt32(totals.signed),
				"failed", atomic.LoadInt32(totals.failed),
			)
		}()

		type signaturePending struct {
			ProposalUUID    string
			ProposalPayload filmarket.DealProposal
		}

		pending := make([]signaturePending, 0, 128)
		if err := pgxscan.Select(
			ctx,
			db,
			&pending,
			`
			SELECT
					pr.proposal_uuid,
					pr.proposal_meta->'filmarket_proposal' AS proposal_payload
				FROM spd.proposals pr
			WHERE
				signature_obtained IS NULL
					AND
				proposal_failstamp = 0
			`,
		); err != nil {
			return cmn.WrErr(err)
		}

		for _, p := range pending {
			wallets[p.ProposalPayload.Client] = struct{}{}

			// Get whether tenant has its peer ID registered as remote signing with filsigner-relayed
			tenantPeerID := new(peer.ID)
			var sig *filcrypto.Signature
			if tenantPeerID != nil {
				// Perform remote signing
				ctx, cancel := context.WithTimeout(ctx, time.Second*10)
				sig, err = filsigner.SignProposal(ctx, *tenantPeerID, p.ProposalPayload)
				cancel()
				if errors.Is(err, context.DeadlineExceeded) {
					atomic.AddInt32(totals.timeout, 1)
					continue
				} else if err != nil {
					atomic.AddInt32(totals.failed, 1)
					continue
				}
			} else {
				raw, err := cborutil.Dump(&p.ProposalPayload)
				if err != nil {
					return cmn.WrErr(err)
				}
				sig, err = gctx.LotusAPI[app.FilHeavy].WalletSign(ctx, p.ProposalPayload.Client, raw)
				if err != nil {
					return cmn.WrErr(err)
				}
			}

			propNode, err := cborutil.AsIpld(&filmarket.ClientDealProposal{
				Proposal:        p.ProposalPayload,
				ClientSignature: *sig,
			})
			if err != nil {
				return cmn.WrErr(err)
			}

			if _, err := db.Exec(
				ctx,
				`
				UPDATE spd.proposals SET
					signature_obtained = NOW(),
					proposal_meta = JSONB_SET(
						JSONB_SET(
							proposal_meta,
							'{ signature }',
							$1
						),
						'{ signed_proposal_cid }',
						TO_JSONB( $2::TEXT )
					)
				WHERE proposal_uuid = $3
				`,
				sig,
				propNode.Cid().String(),
				p.ProposalUUID,
			); err != nil {
				return cmn.WrErr(err)
			}

			atomic.AddInt32(totals.signed, 1)
		}

		return nil
	},
}
