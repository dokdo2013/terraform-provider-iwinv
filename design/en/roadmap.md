# Roadmap and work packages

[한국어](../ko/roadmap.md) · [Index](../../README.en.md) · Revision: 2

No cloud implementation is claimed by this repository's initial commit. Work packages are sequentially gated;
full API/CLI resource coverage remains the destination even though the first release is small.

| Package | Depends on | Deliverable | Completion criterion |
| --- | --- | --- | --- |
| P0 Design baseline | None | Public repo, bilingual design, inventories, checklist, docs CI | Local/remote checks pass; limitations explicit |
| P1 API contract verification | P0 | Redacted read fixtures, auth/encoding/paging/error contracts, remaining surface enumeration, version ADR | G1 passes; gaps have evidence and owner |
| P2 Compute vertical slice | P1 | Go client, Framework provider, catalog lookups, instance lifecycle/import | G2 and G3 pass; no-change plan; signed prerelease candidate |
| P3 Network and storage | P2 + C14–C17 resolution | SG/rules/attachments; storage lifecycle with one connection owner | Each resource passes G4; import and dependency deletion proven |
| P4 Additional control-plane | P1 + per-family contract resolution | Hosting/cache/DBMS/NAS/webmail, products, allowlists, billing, SSH/script lookups | C19–C23 resolved per feature; passwords and destructive replacement documented |
| P5 Service and operation coverage | P1 discovery + relevant P2/P3/P4 | Verified object storage, NAS/cache service APIs, messaging/templates, Actions/Ephemeral, MCP parity audit | Explicit invocation; no secret state leakage; all remote gaps have implementation or documented upstream blockers |
| P6 Stable release and education | P2–P5 evidence | Compatibility/state migration tests, Registry release, Korean/English complete guides | G5 passes; published scope exact; all-coverage claim only after full reconciliation |

Partial prereleases are allowed after P2; label supported scope clearly. Do not wait for every service to begin
user feedback, and do not call a partial release full coverage. Exact dates depend on API access and vendor answers.

## Remaining execution order (as of 2026-09-23)

The development provider registers 18 data sources and 7 resources. The [implementation ledger](../inventory/implementation.json)
defines each capability's live-tested scope; the [checklist](../inventory/checks.json) tracks verification.
**Of 80 checks, 27 passed, 23 are in progress, 28 have not run, and 2 failed.** These counts are not a support percentage.
An authenticated read returned HTTP 200/SUCCESS again after the current test IPv4 address was allowed on the API key.
The test webmail service's cleanup is supported by its authenticated console deletion history and empty service list.
Deletion of the earlier MCP OAuth registration, eligible Compute API zones and instance lifecycle, and Registry publication remain unverified.
One cleaned webmail fixture does not pass the overall cleanup check T056. No account IDs, IP addresses, raw responses or secrets belong in public docs.

| Order | Work to execute | Evidence required to advance |
| --- | --- | --- |
| **#1 / P1** | Reconcile remaining T001–T010 signing, paging, errors and field masks using restored API access. Ask the vendor about this account's Compute API zones/products, visibility of existing instances, the creation restriction, cleanup of the registered MCP client, and required service credentials. Inspect authenticated MCP tools only after registration cleanup and OAuth scope are resolved. | Record eligible zone/product conditions and a disposable lifecycle fixture; distinguish passing and open G1 evidence. Decide T014 after reconciling CLI, service API and MCP gaps. |
| **#2 / P2** | Reconcile the existing Compute read/write adapters with live account contracts. Verify one owned instance's creation ID, completion, Read, update, import, drift and deletion before registering `iwinv_instance`. Never replay an ambiguous create or adopt by name. | Resolve the T015 failure; pass T016–T030 and G2/G3 with live and synthetic evidence distinguished. Record cleanup of every created ID and separately assess retained disks and billing. |
| **#3 / P3** | Retain verified security group/rule support while measuring server attachment add/replace semantics. Verify block-storage creation dependency on an instance, detach/reattach, deletion and retention; choose one connection owner. | Pass T031–T036 and per-capability G4 import, drift and dependency deletion. Do not claim attachment/volume support while instance creation is unavailable. |
| **#4 / P4** | Expand unverified product variants for existing hosting, DBMS, cache and NAS resources. Establish reliable webmail service Read/deletion and a mailbox list/detail substitute; resolve billing-detail access and time-zone gaps. | Meet C19–C23, T037–T042 and G4 for each advertised feature. Do not publish a managed mailbox without reliable Read. |
| **#5 / P5** | Implement object storage, NAS/cache data APIs, messaging/templates, explicit Actions and expiring-value Ephemerals against each service's credentials, errors and ownership contracts. Reconcile verified MCP tools with API/CLI inventory. | Pass T043–T050 and cache T079 per capability; link every remote function to an implementation or verified vendor constraint. Send messages and move data only within an agreed test scope. |
| **#6 / P6** | Review bilingual docs/examples for the exact release scope; prepare production signing custody, Registry public key and least-privilege publication workflow. Sign and publish an immutable version, then verify Registry installation and a real plan in a clean environment. Test previous-state migration once a subsequent version exists. | Assess T051–T056 and G5 from evidence. Reassess T056 only after all work-created resources, including OAuth registration, are reconciled. The first release has no earlier published version, so do not falsely pass T054. |

The immediate execution chain is **#1 vendor eligibility/registration-cleanup inquiry → API contract recheck →
#2 one disposable instance lifecycle**. Synthetic tests and release design can proceed while a vendor answer is pending,
but cannot replace live evidence. A limited prerelease may follow P2 and cleanup gates before every service is complete.
Full API/CLI remote coverage and Korean/English guidance remain the end state across #1–#6.

## Issue preparation

Each package becomes one tracking issue with linked contract IDs (Cxx) and test IDs (Txxx).
Implementation issues must specify scope, dependencies, API evidence, expected plan behavior, import identity,
both-language docs, failure recovery and test cleanup. Use [the capability template](../../.github/ISSUE_TEMPLATE/capability.md).
Initial GitHub issues link these checked-in work packages; documents remain the design source of truth.

## Decisions to record

An ADR records context, alternatives, chosen behavior, consequences, evidence and tests.
Open decisions: exact minimum tool versions; replacement vs in-place transitions; attached-volume ownership;
unreadable creation-only values; collection-wide allowlists; Action state reconciliation; service credential schema.
Never finalize unknown API behavior just to unblock scaffolding.

## GitHub tracking issues

- [P1: API contracts and discovery](https://github.com/dokdo2013/terraform-provider-iwinv/issues/1)
- [P2: Compute vertical slice](https://github.com/dokdo2013/terraform-provider-iwinv/issues/2)
- [P3: Networking and storage](https://github.com/dokdo2013/terraform-provider-iwinv/issues/3)
- [P4: Additional control-plane services](https://github.com/dokdo2013/terraform-provider-iwinv/issues/4)
- [P5: Service APIs, Actions and Ephemeral](https://github.com/dokdo2013/terraform-provider-iwinv/issues/5)
- [P6: Registry and bilingual guides](https://github.com/dokdo2013/terraform-provider-iwinv/issues/6)
