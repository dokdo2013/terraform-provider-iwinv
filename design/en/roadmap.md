# Roadmap and work packages

[한국어](../ko/roadmap.md) · [Index](../../README.en.md) · Revision: 1

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

## Next work: P1

- Confirm a supported account/zone and capture a read-only inventory with pagination.
- Establish signing, transport encoding, success/error envelope, response IDs, nullability and safe field masks.
- Compare instance list/detail with creation schema; design import for missing creation history.
- Enumerate remaining object/NAS/cache service operations and official CLI flags; inspect MCP tools when authenticated.
- Produce ADRs for naming, versions, mutation/replacement semantics, storage ownership and secret handling.
- Specify a disposable lifecycle test environment and recovery/budget rules before moving to P2.

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
