# Webhosting lifecycle design

[한국어](../ko/webhosting-lifecycle.md) · [Architecture](architecture.md) · 2026-09-19

**Design stage. A hosting API adapter exists, but no `iwinv_webhosting` resource or catalog data source is registered.**
Attribute names below are proposals, not executable HCL contracts. Overall service acceptance remains open under T038.

## Why replacement needs special treatment

The public hosting API offers product/server/service reads, creation and deletion, without an update endpoint.
The vendor's deletion documentation states that account deletion is irreversible and the deleted account name cannot be
reused for 24 hours (C29). This is a documented restriction, not a live measurement of the reuse deadline or successful recreation after 24 hours.

Applying `RequiresReplace` to every name/description change could delete a service's data and then fail to recreate it under the same account.
`create_before_destroy` does not establish that two services can share an account name.
The provider must not silently ignore desired changes, invent a private update API, or retry creation for 24 hours.

Before resource registration, establish these policies with Terraform Core tests:

- Changes to readable creation attributes must visibly require replacement instead of pretending to update the remote service.
- Automatic replacement plans should check for an explicitly new account name. Verify safe handling of unchanged and unknown planned names.
- The reuse restriction also affects `-replace`, taint, and recreation after external deletion. Do not promise detection of every Core replacement cause.
- Backup, migration and DNS cutover are separate work. Explain `prevent_destroy` and validate an example that creates a new account before cutover.
- Successful account replacement does not imply automatic migration of web content, databases or domain ownership.

## Proposed attributes and ownership

| Proposed attribute | Remote contract / ownership | Import and change decision |
| --- | --- | --- |
| `id` | Positive integer `service_idx`, preserved as an exact decimal string | Exact-ID import only; no name-based adoption |
| `product_id` | Actual product ID | Restored by Read; replacement candidate |
| `server_id` | Product server catalog integer `idx`, sent as a string | Required for creation, absent from service Read; never fabricated on import |
| `account_name` | API `id`, the login account name, distinct from service identity | Restored by Read; new accounts use 6–12 letters; changes replace |
| `name`, `description` | Literal alias and description | Restored by Read; no public update API; omit empty descriptions on create |
| `web_firewall_enabled` | Y/N `security` | Both values observed in the control plane; packet effects require separate verification |
| `domains` | Domain-to-folder object | Design configured and observed sets separately, without duplicate ownership of the default domain |
| `ftp_password_wo`, `database_password_wo` | Distinct initial passwords | WriteOnly + Sensitive, read from req.Config only, never persist in plan/state |
| `status`, `ip_address` | Control-plane status and IP | Read-only; `active` does not prove HTTP/FTP/database access |
| `timeouts` | Local operation deadlines | Bound visibility/deletion waits and cancellation; do not authorize write replay |

`server_id` records a creation input, not a currently verified setting. Test a schema that allows omission during import while
requiring it for new creation. Adding a selector after import must not silently attest to the remote setting; document its
configuration/replacement semantics explicitly.

Write-only password changes alone cannot produce a plan difference. Decide and test whether an explicit version field triggers
replacement or credentials are limited to initial creation. There is no public password update API, so in-place rotation cannot be promised.
Import must neither require passwords nor synthesize saved defaults.
Follow [HashiCorp's write-only guidance](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments).

## Internal adapter boundary

`internal/services/hosted/webhosting*.go` implements product/server reads, JSON creation, exact-ID selection from a complete list,
and deletion acknowledgement. Product metadata includes PHP choices and domain policy; prices, VAT, disk and traffic units are
excluded until verified. Catalog availability alone does not prove that a particular combination can be created.

- Integer IDs never pass through float64. Invalid IDs are rejected before requests.
- Creation returns identity before validating remaining fields or metadata. Callers must preserve it before processing errors.
- An unambiguous identity in an HTTP 201/202 receipt is returned for recovery, together with an error for the unverified success contract.
- Duplicate/malformed rows or new pagination metadata fail the entire list. A partial list cannot establish absence.
- Names, descriptions and domains are not HTML-decoded. Passwords and unknown response fields are excluded from readable models.
- Explicit empty descriptions are rejected before I/O, reflecting the observed HTTP 422 response; nil omits description.
- Internal password inputs are restricted to distinct 7–20 character printable ASCII values using at least two character classes. This is not exhaustive live validation of all vendor password rules.
- Empty domain maps are rejected until their meaning is verified; nil requests the default domain.
- Deletion acknowledgement and subsequent list absence are separate. API errors/404 never become absence, and writes are never automatically replayed.

## Gates before resource registration

- [ ] Core retains the ID after a failed apply following a create receipt, and destroy can recover it.
- [ ] Neither password appears in plan files, state, private state or diagnostics; ephemeral variables work.
- [ ] Import without historical server choice or passwords supports Read and a no-change plan.
- [ ] Ordinary changes, unknown inputs, taint and explicit replacement expose account reuse risk.
- [ ] Default domains absent from configuration, external domain changes and folder normalization do not cause perpetual diffs.
- [ ] Temporary post-create absence, asynchronous/error states, cancellation and ID preservation are tested.
- [ ] Two-service parallel lifecycles, external drift/deletion, partial failure recovery and exact-ID cleanup pass.
- [ ] Service deletion evidence remains distinct from billing termination; user documentation explains data loss.
- [ ] Korean/English resource guides and examples validated against the actual binary are complete.

[Vendor creation documentation](https://iwinv-hosting.readme.io/reference/웹-호스팅-생성),
[deletion documentation](https://iwinv-hosting.readme.io/reference/웹-호스팅-삭제),
[server choices](https://iwinv-hosting.readme.io/reference/상품-상세-조회).
Live observations are recorded separately in [contract progress](contract-progress.md).
