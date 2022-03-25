spade
==================

This repository contains the software powering the ♠️ (Spade) API:
A Fil-Network Storage-Proposal Management Service.

Spade provides a low-friction environment for Storage Providers (SP) to receive a
virtually unlimited volume of FilecoinPlus (Fil+) denominated storage deals.
It does this by aggregating storage requests and metadata from various sources
(♠️ tenants) and then presenting the aggregate as a single machine- and
human-readable stream to participating SPs. SPs in turn consume this aggregated
list and can trigger an instant storage deal proposal for any entry, as long as
the proposal does not violate the terms set by the originating tenant.

A distinct feature of the ♠️ service is its focus on throughput 🚀 and thus its
design exclusively around a "pull" workflow. A deal proposal can only be initiated
by the receiving SP and all deal proposals are, without exception, made for
**out-of-band data flow** (often mislabeled *offline deals*). This gives **complete
control to the SP operator** over both the deal-transfer mechanism, and the
timing of data injection into their carefully tuned sealing pipeline 😻

From a tenant perspective the service provides a convenient way to disseminate
a dataset to a group of storage providers while strictly following replication
guidelines set by the tenant themselves. In order to become a tenant, all a data
supplier needs to define is a replication policy, a desired list of `PieceCID`s
and their description, and then to make the corresponding `CAR` files available
over HTTP or comparable stream-oriented protocol. The service then takes care of
everything else, even selecting Storage Providers automatically if desired by
the tenant.

[API]: https://raw.githubusercontent.com/ribasushi/spade/master/webapi/routes.go
[#spade over at the Fil Slack]: https://filecoinproject.slack.com/archives/C0377FJCG1L
