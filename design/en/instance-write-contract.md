# Instance write contract preparation

[한국어](../ko/instance-write-contract.md) · [Read boundary](instance-read-contract.md) · [Index](../../README.en.md)

## Internal candidate, not a registered lifecycle

`InstanceWriter` prepares documented create, metadata update and delete requests independently of the read-only `Service.API` interface. It is not called by any registered Terraform resource. No new live write was executed: Compute creation remains blocked by the observed restricted-zone error. Public registration stays at 18 data sources and 7 resources.

Sources checked on 2026-09-19: [create](https://iwinv.readme.io/reference/postv1instances), [metadata update](https://iwinv.readme.io/reference/putv1instancesinstanceid), [delete](https://iwinv.readme.io/reference/deletev1instancesinstanceid), and the [field mask](https://api-kr.iwinv.kr/fields/v1/instances).

## Request and ownership decisions

| Operation | Candidate request | Meaning and remaining gate |
| --- | --- | --- |
| Create | Multipart POST, exact zone/image/flavor IDs, count fixed to 1 | One Terraform owner per instance; no bulk-count input |
| Optional create inputs | Name, description, sorted comma-joined SSH key IDs, existing script ID | Nil omits; duplicate/invalid IDs fail before network; caller slices stay unchanged |
| Metadata update | Multipart PUT with fields=3599; only supplied name/description | Nil omits; explicit empty description is sent but clearing behavior is not yet verified |
| Delete | One bodyless DELETE to the exact ID | Expected 202 plus deleting status is acknowledgement only, never confirmed absence |

Name and description use `url.QueryEscape` once before multipart serialization, following the documented URL-encoding requirement. This candidate uses `+` for spaces; actual server decoding, Unicode length units, allowed characters and normalization remain live gates. A supplied name must be nonempty valid UTF-8; description must be valid UTF-8. The adapter deliberately does not claim to enforce unresolved vendor character/length semantics. Generic client request-size bounds still apply.

Inline block storage, deprecated security-group/public-IP inputs, resize/rebuild/power, readiness polling and import are outside these methods. Their ownership and API contracts remain required work, not silently excluded from the project objective. Omitting SSH keys follows the documented password-access mode, but this adapter does not retrieve or persist a default password; a usable and safe access policy must be settled before resource registration.

## Recovery receipts

A successful creation response uses an array with flat fields, unlike the nested Read model. The adapter extracts valid response IDs before checking expected 202, count=1 and status shape. A sole identifiable row populates `CreateReceipt.ID` even if the remaining acknowledgement is invalid. The caller must persist/journal this before handling the error. Multiple rows never select the first: all valid unique IDs are retained in `RecoveryIDs` for explicit reconciliation. IDs from unsuccessful HTTP or API-error responses are not trusted, and malformed/unparseable bodies cannot manufacture an identity.

No request is automatically replayed. A transport failure may mean the request took effect; no name-based adoption or retry is allowed. Synthetic receipt preservation is not Terraform Core state preservation: the eventual resource must separately prove partial-failure state, interruption and recovery.

Update responses must match exactly one requested ID and the fixed safe read projection. A bad response after a write returns an error and requires reconciliation, not another write. The query-aware multipart client validates the canonical path and signs timestamp+path only. Its TLS-server test verifies query encoding and that pre-encoded multipart text is not encoded twice.

Delete receipts preserve sorted retained block-storage IDs; the documented operation does not delete those volumes. No volume is automatically deleted or adopted. Unexpected status/count/shape or duplicate/invalid volume IDs fails explicitly. Returning a valid `DeletionReceipt` does not clear state, prove absence or prove billing termination. The API-visible list alone cannot establish deletion, because it excludes unsupported zones.

## Validation and registration gates

Synthetic tests cover exact single-create fields, omission/empty distinction, Unicode encoding, input isolation, valid IDs surviving count/status errors, unexpected multiple or malformed rows, safe update projection, retained-volume receipts, invalid paths, cancellation and non-replayed transport/business errors. Fixtures are invented; no vendor example secrets or account responses are committed.

T025 is in progress for this no-replay foundation. T015 remains failed from the actual blocked create attempt; T029 still requires Terraform Core lifecycle evidence. Before registration: resolve the zone gate, prove encoding and create identity on an owned fixture, implement bounded readiness/confirmed-absence contracts, verify safe SSH/script and storage ownership, implement CRUD/import/plan behavior, and run full recovery/cleanup acceptance. No successful instance create/update/delete is claimed.
