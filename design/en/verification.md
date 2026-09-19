# Verification plan

[한국어](../ko/verification.md) · [Index](../../README.en.md) · Revision: 1

**Authenticated read acceptance passed for nine data sources, and lifecycle acceptance passed for security-group attributes and independent ingress/egress rules, plus SHARE PHP 8.4 hosting (T060) and STD Redis DBMS (T063). Compute and remaining managed-resource gates have not passed.**
[Current evidence](contract-progress.md) separates live observations from mock tests.
Documentation CI only checks repository consistency.
The bilingual [checklist](../inventory/checks.json) is the shared executable-work planning ledger.
Each entry has a stable ID, method, expected outcome in both languages, phase and execution status.
`in_progress` means partial evidence, not a pass.

## Evidence and execution rules

Record test ID, commit, provider/Terraform/Go versions, environment class, API/product/zone,
UTC timestamp, input fixture reference, redacted result, pass/fail and cleanup result.
Evidence must distinguish mock contract tests from actual service behavior. A failed or skipped test cannot be marked passed.
Live checks require an explicitly scoped disposable environment, known budget, serial execution where quotas are shared,
and an inventory of IDs created by that run. Never target arbitrary resources by a broad name-prefix delete.
Never execute cloud-changing tests or messaging actions from an untrusted fork pull request.
Do not enable a paid acceptance workflow until its account, egress allowlist, budget and cleanup are configured.

## Gate sequence

1. **G0 documentation:** enumerate API/CLI gaps, classify every known capability and keep both languages synchronized.
2. **G1 read contract:** prove authentication, list pagination, ID mapping, typed Read and existing-account visibility.
3. **G2 lifecycle:** prove one instance lifecycle including import, no-change plan, external drift and delete completion.
4. **G3 failure recovery:** prove throttling, timeout, ambiguous create, cancellation, partial state and cleanup behavior.
5. **G4 family coverage:** repeat lifecycle/import/drift/failure tests for each service; resolve ownership conflicts.
6. **G5 release:** validate examples with a built provider, docs, migrations, compatibility matrix, signed artifacts and clean install.

Implementation may advance by family after its gates pass; unresolved families remain visibly unsupported.
Overall full coverage cannot be declared until remaining service API/CLI/MCP discovery gaps are resolved.

## Required resource test scenario

Create one object -> wait -> refresh -> empty plan -> update mutable field -> empty plan ->
import into separate test state -> verify supported attributes -> matching configuration empty plan ->
change a field outside Terraform -> detect drift -> reconcile -> remove externally -> detect confirmed absence.
Use another independently owned fixture for destroy and dependency-order tests. Do not let two states write one object concurrently.
Replace-only fields must produce an explicit replacement plan; unsupported transitions must fail before mutation.

Use `terraform-plugin-testing` for CLI-driven acceptance, HTTP test servers for protocol/error behavior,
and table-driven tests for schema conversions/signing. Apply race detection to shared clients and limiters.
Document any `ImportStateVerifyIgnore` narrowly with Cxx evidence; it is not permission to hide an import defect.
No network credentials are required for the documentation check or ordinary unit tests.

## Completion evidence

Every released resource/data source/action/ephemeral feature links its capability entry to passing test IDs,
both-language docs, an immutable release and unresolved limitations. Read-only discovery alone is not acceptance.
Cleanup failures are reported with tracked IDs and preserved evidence; never suppress them to make CI green.
Use the [roadmap](roadmap.md) to schedule the checklist. References: [acceptance testing](https://developer.hashicorp.com/terraform/plugin/testing/acceptance-tests),
[import](https://developer.hashicorp.com/terraform/plugin/framework/resources/import),
[Read](https://developer.hashicorp.com/terraform/plugin/framework/resources/read).

## Verification checklist

| ID | Phase | Method | Expected outcome | Status |
| --- | --- | --- | --- | --- |
| T001 | P1 | mock/live-read | HMAC vectors match timestamp+path; query/trailing slash handling is exact | in_progress |
| T002 | P1 | mock/live-read | Clock window errors are diagnosed; every retry signs a fresh timestamp | in_progress |
| T003 | P1 | live-read | Allowed and denied egress IPs produce documented authentication outcomes | in_progress |
| T004 | P1 | mock/live-read | HTTP and business errors are both checked without masking unknown codes | in_progress |
| T005 | P1 | mock/live-read | JSON, form and multipart encoding are proven per operation | in_progress |
| T006 | P1 | mock/live-read | Pagination returns every ID once and terminates on documented conditions | in_progress |
| T007 | P1 | live-read | Account inventory and supported-zone visibility agree with console evidence | not_run |
| T008 | P1 | mock/live-read | Detail fields mask returns required fields without passwords or console tokens | not_run |
| T009 | P1 | mock/live-read | Null, missing, empty and nested array/object shapes decode correctly | in_progress |
| T010 | P1 | mock/live-read | Single lookups reject zero or multiple matches; filters cannot select arbitrary first item | in_progress |
| T011 | P1 | mock/live-read | Provider aliases isolate auth, endpoints and cached data | passed |
| T012 | P1 | mock | Credentials never follow cross-host redirects or enter diagnostics | passed |
| T013 | P1 | review | Version ADR defines tested Go/Terraform/Framework combinations and feature gates | passed |
| T014 | P1 | review | Remaining service APIs, CLI flags and authenticated MCP tools are enumerated with gaps | in_progress |
| T015 | P2 | live | Create returns exactly one stable ID and state preserves it during subsequent failures | failed |
| T016 | P2 | mock/live | Waiters handle building/pending/work/active/off/error with deadline and cancellation | not_run |
| T017 | P2 | live | Repeated refresh and plan after apply produce no changes | not_run |
| T018 | P2 | live | Name/description update and clear round-trip without replacement or perpetual diff | not_run |
| T019 | P2 | live | Korean NFC/NFD, multibyte length, whitespace and special characters round-trip correctly | not_run |
| T020 | P2 | live | Import reconstructs supported fields and matching configuration yields an empty plan | not_run |
| T021 | P2 | mock/live | Unreadable SSH/script history has documented import behavior without fabricated defaults | not_run |
| T022 | P2 | live | External updates are detected; only confirmed external deletion removes state | not_run |
| T023 | P2 | mock/live | 401/403/429/5xx, malformed responses and empty pages never imply deletion | not_run |
| T024 | P2 | mock/live | Creation-time 404 is handled within a bounded consistency window | not_run |
| T025 | P2 | mock/live | Uncertain create outcome is not blindly retried or adopted by name | not_run |
| T026 | P2 | mock/live | Backoff, jitter, Retry-After and aggregate rate pressure stay bounded | not_run |
| T027 | P2 | live | Resize and replacement plans match downtime, address and disk effects | not_run |
| T028 | P2 | live | Destroy waits for absence and reports remaining resources/billing separately | not_run |
| T029 | P2 | mock/live | Partial failure, interruption and rerun preserve IDs and permit documented recovery | not_run |
| T030 | P2 | mock | Parallel resources and clients pass race tests without cross-resource state corruption | not_run |
| T031 | P3 | live | SG direction/protocol/port/CIDR/ICMP and rule ID normalization are verified | in_progress |
| T032 | P3 | live | Rule import, duplicates, external edits and parent deletion have deterministic outcomes | in_progress |
| T033 | P3 | live | SG attachment multiplicity and additive-vs-replacement semantics are proven | not_run |
| T034 | P3 | live | Storage creation attachment, detach/reattach, zone affinity and retention are proven | not_run |
| T035 | P3 | live | Exactly one resource owns a storage connection; deleting server cannot silently lose managed data | not_run |
| T036 | P3 | live | Dependency teardown orders attachments before volume/group/instance deletion | not_run |
| T037 | P4 | live-read | Each additional service has typed response/ID/error and full list contracts | in_progress |
| T038 | P4 | live | Each service supports lifecycle/import/no-change plan/drift tests before release | not_run |
| T039 | P4 | live | Allowlists/referrers prove replace-vs-add, empty-set clear and external-change behavior | in_progress |
| T040 | P4 | live-read | Mailbox accounts are authoritatively readable before managing their lifecycle | in_progress |
| T041 | P4 | mock/live | Password inputs never accidentally leak to logs; state/write-only behavior is explicit | in_progress |
| T042 | P4 | live-read | Billing units, currency, VAT, timezone and sensitive fields are documented accurately | not_run |
| T043 | P5 | mock/live | Object storage signing, addressing, pagination and supported subfeatures pass compatibility tests | not_run |
| T044 | P5 | live | Objects handle hash/ETag/multipart/version/delete semantics without forcing unsupported settings | not_run |
| T045 | P5 | mock/live | Ephemeral console/presign values are absent from plans/state and have tested expiry | not_run |
| T046 | P5 | mock/live | Actions require explicit invocation and never replay on refresh or ambiguous errors | not_run |
| T047 | P5 | mock/live | Rebuild/action results reconcile with later resource Read and configuration | not_run |
| T048 | P5 | live | Messaging template review states and allowed updates/delete behavior are verified | not_run |
| T049 | P5 | mock/live | Send/cancel/timezone/byte limits and duplicate-delivery prevention are verified in scoped tests | not_run |
| T050 | P5 | review | All remaining remote capabilities have tested support or explicit upstream blockers | not_run |
| T051 | P6 | CI | All examples format and validate against the built provider without real credentials | passed |
| T052 | P6 | CI/review | Korean/English pages cover identical behavior and stable contract/test IDs | not_run |
| T053 | P6 | CI | Supported OS/architecture binaries, checksums, signatures and clean Registry install pass | not_run |
| T054 | P6 | CI/live | Previous-version state migration and dependency upgrades keep plans stable | not_run |
| T055 | P6 | CI/review | Untrusted PR CI has no live credentials; release permissions and actions are constrained | not_run |
| T056 | P6 | live | Acceptance run inventory proves cleanup; leaks fail the run with recoverable evidence | failed |
| T057 | P3 | mock/live | Dedicated unattached group attributes pass create/update/import/no-op plan/drift/recreation/destroy; failed create retains an ID for cleanup | passed |
| T058 | P3 | mock/live | Owned TCP/UDP rules pass CRUD/import/no-op plan/direction drift/clear and parent replacements/external deletion and child-before-parent destroy | passed |
| T059 | P4 | mock/live | Hosting adapter catalogs, two-service create/read/delete, peer preservation and exact-ID cleanup pass; Terraform lifecycle and billing remain separate | passed |
| T060 | P4 | mock/live | Hosting Core lifecycle verifies fresh-account replacement, import/no-op without historical inputs, ephemeral write-only plan/state exclusion, failed-create identity recovery and exact-ID cleanup | passed |
| T061 | P4 | mock/live-read | Hosting catalog data sources verify SHARE/SINGLE filters, exact server IDs, sorted complete output, invalid/empty/error cases and live read/no-change plans | passed |
| T062 | P4 | mock/live | DBMS adapter preserves catalog ambiguity and exact IDs, verifies two-service create/read/allowlist replacement, peer preservation and acknowledged exact-ID cleanup | passed |
| T063 | P4 | mock/live | STD Redis Core lifecycle passes authoritative allowlist update/drift, import/no-op without account history, fresh-account replacement, failure identity retention and exact-ID cleanup | passed |
