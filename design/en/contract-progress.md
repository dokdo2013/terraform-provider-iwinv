# P1 contract verification progress

[한국어](../ko/contract-progress.md) · [Verification](verification.md)

Observed 2026-09-19 UTC. This is an implementation checkpoint for [issue #1](https://github.com/dokdo2013/terraform-provider-iwinv/issues/1),
not a stable release or completion of P1. See the [development guide](development.md) for the working zone data source. The client currently supports a single GET attempt.

## Implemented and tested

- Control-plane HMAC-SHA256 with a fresh timestamp per call, query excluded, canonical ASCII paths only.
- TLS verification, all redirects rejected, 30-second HTTP timeout and 2 MiB response limit.
- Per-client request admission at one request per second. No cross-process quota guarantee.
- HTTP and business-envelope errors checked independently. Unknown success representations fail closed.
- No response bodies, server messages, URLs or credential values in returned errors.
- Raw JSON preserves missing/null/empty values until a service decoder validates them.
- Synthetic tests cover signatures, Korean query encoding, errors, redirects, cancellation,
  client isolation, concurrent calls, response limits, and value-free probe reports.

The client does **not** implement writes, retries, pagination or resource deletion inference yet.
It preserves HTTP status, including 202, for service-specific interpretation.
The probe accepts only HTTP 200 and an array of zone objects.

## Authenticated observations

The temporary, IP-restricted key was used for read-only requests. No credentials, account payloads,
actual resource IDs or state files are included in this repository.

| Operation | Observation | Remaining gap |
| --- | --- | --- |
| Zones | HTTP 200; `zone_id`, `zone_name`, `status` strings; numeric count | Console-wide account/zone comparison |
| Flavors | Four pages with 10/10/10/3 results; 33 distinct IDs; numeric `total` | Behavior while catalog changes |
| Images | Five pages with 10/10/10/10/0 results; 40 distinct IDs; no `total` | Detail mapping and exact-filter behavior |
| Beyond final page | Flavors/images return HTTP 200, empty array, count 0 | Other services need independent tests |
| Instance list | HTTP 200 with empty result for the tested account/API scope | No existing instance detail/import evidence |
| SSH key list | HTTP 200, array, string key IDs; numeric pagination metadata | No key mutation API is documented |
| Stale timestamp | A signed zones request 600 seconds in the past returns HTTP 401 | Exact boundary and dedicated error classification |

These observations provide partial evidence for T001, T002, T003, T004, T006 and T009.
T011 includes mock isolation and a native binary test separating a valid-key read from a synthetic-invalid alias. Redirect/error redaction tests satisfy the
current mock-only T012 scope. None of this proves resource lifecycle acceptance.

## Run locally

```sh
go test -race -cover ./...
go vet ./...
python3 scripts/check_docs.py
```

For the optional authenticated probe, inject `IWINV_ACCESS_KEY` and `IWINV_SECRET_KEY` through your
local secret manager/environment, then run:

```sh
go run ./cmd/contract-probe
```

The probe makes exactly one GET request to `/v1/zones`. Its output contains allowlisted field types,
not remote IDs/names or secret values. It does not read CLI profiles, accept endpoint overrides,
write state, create resources or claim acceptance success. An error exits nonzero.
Keep keys out of shell history and revoke temporary credentials after the test session.

Authenticated mutation tests must track only IDs created by that run, use serial operations,
and verify cleanup. An empty account list never authorizes deleting arbitrary objects.

## Compatibility decision (ADR-0001, provisional)

Use Go 1.25.8 as the module minimum, with CI on 1.25.8 and 1.26.1. The client itself uses the standard library; provider dependencies are pinned in go.mod.
Local contract tests run on Go 1.26.1, macOS arm64. Terraform 1.14.2 passed synthetic protocol tests
and authenticated zone data-source acceptance with a subsequent no-change plan.

Framework 1.19.0 and plugin-testing 1.16.0 are pinned and compiled, matching the inspected
[official scaffolding dependency baseline](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/go.mod).
These dependencies have passed the local protocol and zone acceptance tests.
Adopt Terraform >=1.14 for the initial provider compatibility target and test the minimum separately before release.
[Ephemeral resources](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources)
require Terraform >=1.10 and [write-only arguments](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments)
require >=1.11; those feature floors do not lower this project's intended baseline.
Actions require their own acceptance and state-reconciliation tests before publication.
The T013 provider/protocol matrix passed CI with Go 1.25.8/1.26.1 and Terraform 1.14.0/1.14.2.
[Actions](https://developer.hashicorp.com/terraform/plugin/framework/actions/testing) require Terraform >=1.14; this does not claim an implemented action.

Sources: [iwinv signing](https://iwinv-common.readme.io/reference/api-request),
[response envelope](https://iwinv-common.readme.io/reference/api-response),
[zones](https://iwinv.readme.io/reference/getv1zones),
[flavors](https://iwinv.readme.io/reference/getv1flavors),
[images](https://iwinv.readme.io/reference/getv1images).

## Additional discovery and first mutation experiment

The official [CLI v0.2.2 command/flag inventory](../inventory/cli.json) contains 47 root/subcommand help surfaces,
including the root command and completion commands. It was inspected without logging the CLI into an account.
The installer targets `/usr/bin`; the audit instead used a temporary binary without installing it system-wide.

[Additional service operations](../inventory/service-operations.json) record separate NAS, Cache, Swift and S3 surfaces.
Object Storage documents both protocols and separate keys, with the public endpoint `kr.object.iwinv.kr`:
[authentication](https://help.iwinv.kr/manual/712), [compatibility](https://help.iwinv.kr/manual/738).
The [visual transcription ledger](../inventory/object-compatibility.json) records vendor claims for 22 S3 and 22 Swift features; live verification remains separate. A CLI `obs://` URL is not proof of full S3 compatibility.
The vendor excludes the `x-amz-security-token` request header and version/delete-marker response headers, requiring explicit STS/versioning decisions.
Swift temporary URLs exclude container keys, and versioning excludes `X-History-Location`.
S3 policy syntax may legitimately contain AWS-style ARN strings; this does not imply iwinv control-plane IAM/ARN support.
Swift and S3 must not manage the same bucket/object through competing Terraform resources.

The [NAS manual](https://help.iwinv.kr/manual/763) has a consequential inconsistency: rename/tag updates are PUT in
its summary but DELETE in detailed tables. Those methods remain unresolved; the provider must not infer DELETE for rename.
[Cache](https://help.iwinv.kr/manual/938) also has tenant-scoped token authentication, distinct from control-plane HMAC.
The generic SDK landing page does not identify a downloadable SDK package.
Authenticated MCP discovery still needs OAuth; a control-plane key is not substituted for a bearer token.
These are partial T014 results, not proof of full coverage.

Read-only control-plane checks reached all five additional service catalogs. Their service lists were empty in the tested account.
Product lists returned arrays without standard `count`/pagination fields; Cache included a null `product_id`.
An initial hosting server catalog request without `product_id` and the user-script list returned 404. The hosting catalog later succeeded with its required product query (see below); the script endpoint remains unresolved. An empty successful list and a 404 cannot be normalized indiscriminately.
Block-storage list returned HTTP 200 although older documentation advertises 202.

For T005/T015, one explicitly scoped multipart server create used a small Linux flavor, a compatible zone/image and one existing SSH key reference.
It returned HTTP 500, `DEV_CHECK_RETURN`, and no instance ID. The result was an error object, not the documented success array.
The [official error catalog](https://api-kr.iwinv.kr/error) identifies this as a server-return problem, not an authentication error.
No automatic retry or name-based adoption occurred. Immediate and delayed instance lists were empty.
The cause, backend outcome and billing state are unverified, so compute CRUD remains gated.
T015 is failed for this API contract experiment; no Terraform instance resource acceptance was claimed.

[CI evidence](../inventory/evidence.json) links source commits to successful workflow runs.

## Image and product read implementation evidence (2026-09-19)

Added `iwinv_images`, `iwinv_image`, `iwinv_instance_types`, and `iwinv_instance_type`.
Read-only Terraform acceptance verified complete catalogs, public-image detail, product detail, and a subsequent empty plan.
At page size 10, images returned 40 entries followed by an empty page; products returned total 33 with a final three-entry page.
These are observations from this run, not constants embedded in the implementation or acceptance assertions.
For synthetic nonexistent IDs, image detail returned HTTP 400 / `ID_INVALID`, while product detail returned HTTP 200 / an empty array.
Both are lookup failures. Singular reads also require exactly one result with the exact requested ID.

[Image detail](https://iwinv.readme.io/reference/getv1imagesimageid) documents differences between public and private images.
Private images and the complete detail fields remain unverified; outputs are deliberately limited.
[Product detail](https://iwinv.readme.io/reference/getv1flavorsflavorid) IDs can contain dots, which are preserved.
Synthetic tests cover duplicates, malformed metadata, changing totals, late failures, page bounds and cancellation.
These checks improve catalog consistency; they do not resolve create failures or establish pagination contracts for other services.

## Security group and rule API observations (2026-09-19)

Independently of the server create failure, three unattached test security groups were created sequentially for contract checks.
No existing groups or servers were modified. Exact string IDs from create responses were recorded in a private cleanup journal.
These are direct API experiments, not Terraform security-group resource implementation, acceptance or import completion.

| Operation/input | Observation | Implementation implication |
| --- | --- | --- |
| Group create/detail/update | All HTTP 200; string `firewall_id` | Do not copy documented 202/integer-create-ID examples into the model |
| Group name/description/ICMP update | Read matches submitted values, including `N` to `Y` | Explicit write followed by Read can verify values |
| Rule create/update | HTTP 200; integer `rule_id` | Explicitly design conversion to Terraform string identity |
| `IN`/`TCP`, single port, /32 | Create and subsequent read succeeded | Use the verified uppercase values |
| Documented `inbound`/`tcp` combination | HTTP 400 / `CHECK_PARAM_ENUM` | This combined test does not isolate which individual value caused rejection |
| `OUT`/`UDP`, `10000-10002`, /24 | Create succeeded with submitted values preserved | Partial evidence for ranges and outbound direction |
| `192.0.2.1/24` | Success response retained host bits in the submitted text | Silently masking the address may cause state inconsistency; packet behavior is unverified |
| PUT only rule port/description | Omitted direction/protocol/IP/name retained | Record observed partial-update behavior |
| Rule DELETE followed by parent rule list | HTTP 200 / empty array / count 0 | Individual rule absence verified |
| Group DELETE followed by detail | HTTP 200 / empty array / count 0 | Candidate endpoint-specific absence contract; not every 200 means absence |
| Rule list after deleting parent with rules | HTTP 400 / `CHECK_PARAM` | This error alone is not absence; first establish exact parent absence |

All test groups were absent from both detail reads and the complete group list after delete acknowledgements.
One rule was independently deleted and verified absent. The other two became unaddressable after parent deletion,
which does not establish physical backend cascade deletion. No test group was attached to an instance.
Duplicate rules, lowercase values individually, IPv6, ICMP packet behavior, import, external edits and attachment cardinality remain unverified.
This is partial evidence for C15/C16 and T031/T032, not completion.

Sources: [group create](https://iwinv.readme.io/reference/post_v1-security-groups),
[group detail](https://iwinv.readme.io/reference/get_v1-security-groups-id),
[rule create](https://iwinv.readme.io/reference/post_v1-security-groups-id-rules),
[group delete](https://iwinv.readme.io/reference/delete_v1-security-groups-id).

## Create restriction diagnosis and write client (2026-09-19)

After rechecking catalog availability and the absence of the earlier create name in delayed inventory,
one separate minimal multipart diagnostic request omitted description, SSH keys and scripts.
It again returned HTTP 500 / `DEV_CHECK_RETURN`. The nested result had numeric code 12 and
a Korean message stating that resource use is restricted in the selected zone. Embedded executable text was neither executed nor exposed in provider diagnostics.
This establishes a create restriction but does not distinguish account policy from zone policy; vendor clarification is required.
No create ID was returned and the immediate API inventory was empty. C05/T015 remain unresolved.

The shared Go client now supports JSON/multipart writes and DELETE.
Synthetic tests cover signing, empty-versus-omitted fields, Korean/special-character transport, size bounds, redirects, and no replay after HTTP errors or a dropped connection.
The actual Go client created, updated and deleted separate test security groups, verifying empty detail results after deletion.
In these ASCII-only tests, PUT with an empty description returned HTTP 200 but retained the previous description. The initial test expecting a successful clear failed.
Later escaped-text tests below invalidate treating this as a general no-op contract; the adapter now rejects empty updates.
A future resource must not record a successful clear in state. All Go test groups were verified absent after deletion.

## Partial NAS control-plane lifecycle (2026-09-19)

One temporary NAS used an available minimum-disk product, changed its allowed IPs, and was deleted.
Despite the documented `array(string)` type, creation accepted a JSON object such as `{ "192.0.2.1": "RO" }` for `allowip`.
The addresses below are documentation-only synthetic addresses, not access addresses or account identifiers.

| Item | Observation | Remaining verification/design implication |
| --- | --- | --- |
| POST JSON | HTTP 200; result is one object; integer `service_idx` | Do not reuse an IaaS result-array decoder |
| GET list | Result array without count/page; exact ID/settings observed | Complete inventory and external-change consistency need further testing |
| Status | `pending` on create/immediate Read | HTTP 200 does not prove storage readiness; ready/mount/billing remain unverified |
| `allowip` | Read also returns an IP→RW/RO object | Evidence for map ownership design |
| PUT `{ "192.0.2.2": "RW" }` | Old IP removed; only new IP remains | One resource must own the complete allowlist |
| PUT empty object | HTTP 422 with `message`/`errors.allowip`; old value retained | Empty-set clearing observed as unsupported; not a standard success envelope |
| Read fields | Integer `spec.disk`, null `stop_date`, string `mount_info` | Actual domain/mount values stay out of public fixtures/logs |
| DELETE | HTTP 200; string result; exact ID absent in subsequent list | This test NAS was verified deleted |

This is partial evidence for C19–C21 and T037/T039. The test did not await completed provisioning, mount storage or write data.
Tenant service authentication/file operations and Terraform lifecycle/import remain unverified. No NAS resource is implemented yet.
[Official NAS create documentation](https://iwinv-api-nas.readme.io/reference/%EA%B3%B5%EC%9C%A0-%EC%8A%A4%ED%86%A0%EB%A6%AC%EC%A7%80-%EC%83%9D%EC%84%B1).

## Hosting, Cache and DBMS contract experiments (2026-09-19)

These scoped API experiments continue C19–C22 and T005/T037/T039. They do not register Terraform resources or satisfy T038 lifecycle/import acceptance.
Every successfully created hosting, Cache and DBMS parent was recorded privately by exact create ID, then deleted and verified absent from its service list.
No existing resources, production databases or message recipients were changed.

| Service | Live observation | Design implication / unresolved limit |
| --- | --- | --- |
| Hosting server catalog | `GET /v1/webhosting/servers?product_id=…` succeeds; rows have integer `idx` and PHP/database metadata | The earlier unfiltered 404 was not evidence of an unavailable endpoint |
| Hosting create/read | JSON create succeeds; object `result` with integer `service_idx`; list transitions `pending` → `active`; Korean and `& + %` description preserved | Read omits `server_idx`/PHP selection history; import cannot reconstruct these inputs from this response |
| Cache password | Documented `ftppw` returns 422 requiring `pw`/`pw.FTP`; scalar `pw` also fails; JSON `pw: {"FTP": "…"}` succeeds | Use the verified nested input; never publish the real generated password |
| Cache create/read | Create omits `product_id`, Read includes it; `pending` → `active`; create-time `allow_referer` is ignored | Persist the create ID before full validation; apply owned referrers separately after creation |
| Cache referrer PUT | JSON array replaces the whole list; one and two entries read back exactly | One owner per whole list; the endpoint name “add” does not establish append semantics |
| Cache query encoding | Plain query array returns 422; bracketed query array without a body returns 400 / `REQUIRED_POST_PARAM_MISSING` | Do not use query-only updates for this observed contract |
| Cache empty array | JSON empty referrers return 422 and keep the old list | Clearing is unresolved for a managed resource; do not save an empty successful state |
| Cache consecutive writes | An immediate second PUT returns 404 / `NOT_FOUND` with a busy-operation message; the parent still exists | Never turn this error into absence or blindly retry the write |
| Cache spaced writes | With an extra delayed Read between writes, both replacements succeed; observed status remains `active` | `active` alone does not prove the write lock is released; no reliable lock/readiness contract yet |
| DBMS catalog | Multiple rows reuse a `product_id` across engine versions; Redis-filtered selection used a unique ID | Do not silently deduplicate or promise version selection; CPU units remain unverified |
| DBMS create/read | JSON Redis create succeeds; `waiting` → `active`; create `domain` is a string, Read `domain` is an object | Keep create receipts separate from read models; this does not prove database connectivity |
| DBMS allowip | JSON array wholly replaces the list; empty array returns 422 and retains the old IP | Authoritative-set ownership, with an explicit unsupported-clear decision before release |

Sources: [hosting server catalog](https://iwinv-hosting.readme.io/reference/상품-상세-조회),
[Cache create](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-생성),
[Cache referrers](https://iwinv-cache.readme.io/reference/레퍼러-추가),
[DBMS products](https://iwinv-dbms.readme.io/reference/클라우드-dbms-상품-조회).
These links describe vendor intent; the table distinguishes measured differences.

## Webmail visibility and credential observations (2026-09-19)

A temporary `.invalid` domain was used; no DNS records were changed and no mail was sent.
Create returned HTTP 200, an integer `service_idx`, and `pending`. The first immediate service list was empty.
A separate bounded experiment preserved the returned ID and waited: the service appeared in a later list as `active`.
An empty immediate list therefore cannot justify dropping create identity or retrying creation.

The parent Read omits `name` even though create returns it. Account creation returns HTTP 200 and **echoes the submitted password**;
the returned `service_idx` is a string. Neither the account ID nor an account collection appeared in the subsequent parent list.
Account deletion was acknowledged, but the documented API offers no independent account Read to verify absence.
The following parent DELETE returned 404 / `NOT_FOUND` with a busy-operation message; delayed parent lists were empty.
A later explicit cleanup DELETE also returned 404 and subsequent lists remained empty. Console cancellation and billing termination are unverified.
This is a cleanup/visibility ambiguity, not evidence that the parent was never created or that child deletion safely cascades.
T040 remains in progress: authoritative account Read/import is unresolved. T041 remains in progress: redaction is tested, but Terraform write-only/state behavior is not implemented.

The internal `hosted` decoder preserves integer identity without float rounding, extracts create identity before validating other fields,
allows absent webmail names, rejects malformed/duplicate list records and changed pagination metadata, and excludes arbitrary credential fields.
Its `Find` result describes only the observed list; service-specific consistency and deletion rules are still required.
Six private create/list response pairs from hosting, Cache, DBMS and webmail were also decoded and matched by exact ID without emitting payloads.
Only synthetic fixtures are committed. This helper is not a claim of an implemented hosted-service resource.

Sources: [webmail create](https://iwinv-webmail.readme.io/reference/웹-메일-생성),
[account create](https://iwinv-webmail.readme.io/reference/웹-메일-계정-생성),
[service list](https://iwinv-webmail.readme.io/reference/웹-메일-서비스-조회).

## SSH key reference implementation (2026-09-19)

`iwinv_ssh_keys` and `iwinv_ssh_key` now implement the read-only portion of C18.
The authenticated API returned string `ssh_key_id`/`name`/`start_date`, numeric count/page metadata and no total.
With page size 1, the existing single key filled page 1 and pages 2/3 returned empty arrays with matching page numbers.
The implementation requests 10 per page; live Terraform acceptance read the existing key and passed a subsequent no-change plan.
Only IDs and names enter state. No keys were created, downloaded, changed or deleted, and no server was accessed.

Synthetic tests cover multiple full pages followed by an empty page, stable ID ordering despite duplicate names,
missing IDs, late errors after an early match, malformed metadata, duplicate IDs, changed response metadata,
cancellation, page bounds, empty Terraform collections and omitted unknown key-material fields.
The exact-ID data source validates every page before returning a match. It never selects by name or uses an invented detail endpoint.
These results add evidence for T006/T009/T010; they do not complete contracts for unrelated APIs.
Key creation/deletion and server SSH installation remain unresolved or unverified, and no managed key resource is claimed.

Source: [SSH key list](https://iwinv-common.readme.io/reference/get_new-endpoint-1-1).

## Typed security-group adapter and corrected contracts (2026-09-19)

The internal network service now implements group list/detail/create/update/delete contracts. This is P1 contract preparation;
no Terraform group/rule/attachment resource is registered, and P2/P3 gates remain open.

The first adapter live run failed **before creation** because the list includes `page_no`/`page_size` omitted from earlier shape summaries.
The API default was 50; explicit page size 1 and page numbers 1/2 were echoed on empty inventory.
Nonempty listing with the typed adapter used page size 50. Detail/create/update have count but no page metadata.
The list implementation validates and traverses pages, rejects duplicates/late errors, and bounds traversal at 1,000 pages.
Synthetic tests exercise full pages and termination; a live account with more than one page of groups has not been tested.
The discovered OpenAPI lists no pagination parameters for this endpoint, so these remain live observations rather than edited vendor claims.

| Contract | Evidence and implementation |
| --- | --- |
| Group identity | Validate an exact `FIREWALL-…` path segment; a create receipt with one ID preserves it even if other fields/count/status fail validation |
| Detail absence | Only HTTP 200 plus an empty array and count 0 becomes absent; 404, business errors, mismatched IDs and malformed results remain errors |
| Description transport | JSON sends literal input. Responses escape description as HTML; the adapter decodes it exactly once. Name is preserved verbatim |
| Encoding probes | Quotes, angle brackets, ampersands, literal `&amp;`/`&#39;`/`&lt;`, Korean and decomposed Unicode round-trip after one description decode; URL-encoding is stored literally and must not be applied |
| Empty description update | Earlier ASCII-only tests retained the previous value. A later test with escaped text failed preservation, so empty updates must not be treated as harmless no-ops; the adapter rejects them before I/O |
| Omitted update description | Scoped API and typed Go tests preserved the existing description while changing name/ICMP; omission is distinct from clearing |
| Create without description | An exploratory request produced no usable receipt; that harness did not retain the error classification. Later inventory was empty. This does not establish that the field is required; omission remains unresolved |
| Delete | One request validates acknowledgement only; a separate exact-ID detail Read verifies absence |

After correcting pagination and description decoding, the live Go adapter test passed create receipt validation,
list/detail reads, Korean/special-character updates, rejected-clear preservation, omitted-description rename/ICMP update,
and deletion followed by absence. Three Go groups with recorded IDs and two focused encoding-probe groups were verified deleted.
The failed no-ID create was not adopted by name or automatically retried; a later inventory contained no matching test name.
All raw responses, real IDs and cleanup journals remain private. Synthetic tests also cover partial create identity,
ambiguous result counts, invalid paths, cancellation, bounded pagination, null/empty/missing fields, and one-attempt errors.
The test harness now retains safe create-error classification and registers cleanup before persisting a known create ID.

These are additional T005/T006/T009/T010 and C15 findings, not Terraform state/import, attachments or rule-cascade acceptance.
Sources: [group list](https://iwinv.readme.io/reference/get_v1-security-groups),
[group detail](https://iwinv.readme.io/reference/get_v1-security-groups-id),
[group create](https://iwinv.readme.io/reference/post_v1-security-groups).

## First managed resource: security-group attributes (2026-09-19, T057)

`iwinv_security_group` is now registered as a development resource. Its supported fields are name, nonempty description and ICMP,
with exact-ID import and operation timeouts. The [bilingual resource guide](../../docs/resources/security_group.md) specifies scope and recovery.
This advances the independent network surface while instance creation remains blocked; it does not close P1/P2/P3 or the six implementation issues.

Before choosing the description default, two diagnostic creates with omitted/empty content returned HTTP 403 / `CHECK_IP`,
with a nested temporary-failure message and no ID. Follow-up inventories did not show either requested name.
The process egress matched the allowed IP, and a nonempty-description positive control succeeded and was deleted with exact-ID absence confirmed.
This is a bounded observation, not a new general interpretation of `CHECK_IP`: the official error catalog describes an IP restriction.
The provider therefore sends a verified nonempty description (`Managed by Terraform` when omitted in HCL) and rejects explicit empty descriptions.
The earlier omitted-description observation remains without a retained error classification; it is not retroactively assigned this result.

Two live Terraform runs passed with Go 1.26.1 / Terraform 1.14.2. Four returned create IDs were privately recorded and all four exact-ID absences verified.
The second run additionally relinquished Terraform ownership without destroying the object, imported into the same persisted state and verified an empty plan.
Both runs covered create/read/update with stable ID, full import attribute comparison, Korean/literal-entity descriptions,
no-change plans, external drift detection/repair, external deletion/recreation and final destroy. No existing group was mutated.
The positive control above was separate from these four Terraform creations and was also deleted.
The private journals and raw state/logs are intentionally excluded from this repository; public fixtures use synthetic IDs only.

Synthetic Terraform CLI tests also passed for defaults, timeout-only updates with zero API writes, invalid empty configuration,
and a malformed create receipt that retained its ID through a failed apply and subsequent destroy without a second POST.
Direct framework tests cover create visibility delay/timeout, prior-state preservation on update/delete failures,
HTTP authentication/not-found/rate-limit/server errors, and endpoint-specific absence.
Evidence: `internal/provider/security_group_resource_test.go`, `internal/provider/security_group_live_test.go`, and the network adapter tests.

The ledger marks only POST/GET-detail/PUT/DELETE for this resource; internal/test-only group listing is not exposed as a Terraform capability.
Live evidence does not include rules, attachments, packet behavior, API length boundaries, full real pagination or billing closure.
The compute zone restriction and prior webmail console/cancellation uncertainty remain unresolved. These group cleanups do not certify all earlier service cleanup.

## Independent ingress/egress rules (2026-09-19, C16, T058)

Registered `iwinv_security_group_ingress_rule` and `iwinv_security_group_egress_rule` as development resources.
The [shared guide](../../docs/guides/security_group_rules.md) defines ownership, exact compound import IDs, replacements and recovery.
P2 compute and the complete P3 gate remain open; supporting rules does not establish instance attachments, storage or packet filtering.

Additional raw API experiments established these scoped contracts:

- Rule `title` and `content` return verbatim, including Korean and literal HTML entities. Do not reuse group-description HTML decoding.
- Empty and omitted description creates return null. Empty/null description updates retain existing text; omitted updates also retain it.
- Direction, protocol, port range, CIDR, title and nonempty content changed in place while retaining the integer rule ID.
- Exact duplicate creation returned HTTP 400 / `CHECK_PARAM`. Independently tested lowercase `tcp` and `inbound` returned `CHECK_PARAM_ENUM`.
- IPv6 returned `IPV6_NOT_SUPPORTED`; `ICMP` returned `CHECK_PARAM_ENUM`. Port `0` and range `0-65535` returned `CHECK_PARAM`; `65535` succeeded.
- A bare-IP update failed with `CHECK_PARAM`; the otherwise equivalent CIDR update succeeded. Only IPv4 CIDR input is supported.
- A 53-rule fixture returned every recorded ID in a plain list and both `page_no=1/2,page_size=1` requests. No pagination metadata was present; query pagination was ignored.
- Raw probes initially exposed a null-description assumption in the harness and a rejected bare-IP update. Both interrupted runs cleaned their recorded children and parent; they are not represented as successful full experiments.

The typed adapter validates all rows before exact-ID lookup, rejects changed pagination/count contracts, preserves int64 IDs without float64,
and keeps a known create identity even when later receipt fields fail validation. Rule Read checks parent detail first:
parent success/empty establishes parent absence, whereas a rules `CHECK_PARAM` or 404 alone cannot remove state.
Writes are never automatically replayed; empty update clears are rejected before I/O.

The Go adapter live test passed for create/read/full update/omitted-description update/individual delete with peer preservation and cleanup.
Terraform live acceptance with Go 1.26.1, Terraform 1.14.2 and the race detector passed both direction resources,
full import comparison, persisted re-import/no-op plans, direction drift repair and description-clear replacement for both directions, plus ingress parent-change replacement,
external rule deletion/recreation and child-before-parent destroy. The ownership wrapper refuses parent deletion before each known child's absence is verified.
All seven parents and 66 individually recorded rules across these raw/Go/Terraform runs were deleted; rule absence was checked while parents still existed.
Private receipts, IDs, state and logs are excluded from Git. These cleanups do not resolve the earlier webmail cancellation/billing question.

Synthetic Terraform tests additionally passed failed-create ID cleanup without another POST, timeout-only changes with no update request,
invalid input rejection before parent creation, parent-absence handling, failed update/delete state preservation,
and conservative replacement for an unknown new description when the prior description is nonempty.
Known nonempty updates remain in place; unknown values can no longer introduce an unapproved replacement only during apply.
Both group and rule creates reject runtime-unknown timeout values before writing, so unknown state cannot invalidate a returned identity.

Evidence: `internal/services/network/rules_test.go`, `internal/client/rules_live_test.go`,
`internal/provider/security_group_rule_resource_test.go` and `internal/provider/security_group_rule_live_test.go`.
T031/T032 remain partial because packet behavior, attachments, full boundary variants and physical cascade behavior are not all verified.
Official contracts: [list](https://iwinv.readme.io/reference/get_v1-security-groups-id-rules), [create](https://iwinv.readme.io/reference/post_v1-security-groups-id-rules),
[update](https://iwinv.readme.io/reference/put_v1-security-groups-id-rules-rule-id), [delete](https://iwinv.readme.io/reference/delete_v1-security-groups-id-rules-rule-id).

## Typed hosting adapter and replacement constraints (2026-09-19)

Added product/server catalogs, exact-ID selection from a complete service list, JSON creation and deletion acknowledgement.
No Terraform resource or data source is registered. This partially supports C19/C29 and T037/T041;
T059 passes only for the adapter scope below. T038 and password plan/state verification remain incomplete.

- Verified the unfiltered product catalog against its SHARE/SINGLE partitions and selected PHP 8.4 through the required product_id server query.
- The first experiment created the default-domain service, observed active status and verified acknowledged deletion and absence, but its second create failed.
  Isolated input probes identified explicit empty description as HTTP 422/errors.description. Omission reads back as an empty string.
  The adapter rejects explicit empty descriptions before I/O and preserves the distinction from omission.
- Firewall N and a custom `.invalid` domain each passed separate create/active/delete probes. The corrected Go race contract test created two services together
  and verified Y/N, default/custom domains and omitted/literal Korean `&amp; + %` descriptions. Names and descriptions are not HTML-decoded.
- A service requested with one custom domain returned two domain mappings including its default. Configured and observed maps cannot simply replace one another;
  the [lifecycle design](webhosting-lifecycle.md) separates their ownership.
- Both created IDs appeared in the shared list, and deleting one preserved the other in active status.
  All five hosting services created in this stage had privately recorded exact IDs, HTTP 200 deletion acknowledgements and subsequent verified absence.
  Neither account from the initial failed run appeared in a later list; no name-based adoption or automatic create replay occurred.
- Synthetic tests cover IDs above 2^53/int64 handling, malformed/duplicate/partial lists, HTTP/metadata changes, nullable descriptions,
  recovery identity after unverified create responses, safe diagnostics and request encoding. Prices, VAT and storage units remain outside the model.

C29's 24-hour reuse restriction comes from the vendor's deletion documentation; recreation after that interval has not been live-tested.
Terraform import/replacement/external drift/password non-persistence, HTTP/FTP/database connectivity and billing termination remain unverified.
These hosting cleanup results do not resolve the earlier webmail console cancellation/billing uncertainty.

Sources: [hosting deletion](https://iwinv-hosting.readme.io/reference/웹-호스팅-삭제),
[hosting creation](https://iwinv-hosting.readme.io/reference/웹-호스팅-생성).

## Webmail API absence contradicts console presence (2026-09-19)

In the authenticated console, the earlier test service's exact ID, name and domain matched its private creation record.
The list still showed **working, zero accounts**, while the detail page showed **active**, with its deletion control disabled.
At the same time, `GET /v1/webmail` returned HTTP 200/SUCCESS with an empty array.
A successful empty API list therefore cannot establish deletion/cancellation for this service.
The provider must not call console endpoints or bypass disabled UI controls.

T056 is marked `failed` because this work-created service remains uncleaned. This does not invalidate other tests' documented deletion
evidence, but the overall cleanup gate is not satisfied. C23/T040's reliable Read contract also remains unresolved.
A private vendor-support draft requests cancellation of this test service only, billing termination confirmation and an explanation of the API omission.
No external message has been sent; separate user authorization was requested.
Actual IDs, account/domain names, login information and raw console output are excluded from the public repository.

## Terraform hosting lifecycle (T060, 2026-09-19)

The development provider now registers `iwinv_webhosting`; the previous internal-only limitation is superseded for this resource.
Product/server catalogs remain internal. Live acceptance used Go 1.26.1, Terraform 1.14.2 and `go test -race` and passed in 62.53 seconds.
The run selected a SHARE product supporting custom domains and a PHP 8.4 server. Inputs, exact IDs, intent/receipt journals and logs
remain private; no existing service was mutated.

Four owned service identities were created and all four received deletion acknowledgements followed by exact-ID absence.
The test maintained two services concurrently, verified initial no-change plans and full readable-attribute import, then created a fresh
account with custom `.invalid` domain, literal description and firewall N before deleting its predecessor. The other service remained
active. Ownership was relinquished without deletion, then persistently re-imported without historical server/password/version inputs;
Read and a no-change plan passed. An external deletion was detected and a new account was created, followed by final cleanup.

Actual compressed saved plans and state were inspected for the ephemeral initial passwords, in addition to parsed plan checks.
Synthetic Core tests passed malformed-create receipt recovery with the ID retained, fresh-account destroy-before-create replacement,
unknown changed attributes, missing replacement inputs, domain drift and local timeout-only updates. Taint/explicit `-replace` tests
inspect destructive plans only and do not perform same-name recreation. Direct resource tests cover hidden creates, deadline expiry,
API errors and failed deletion state retention. Ordinary replacements require a known different account, server and initial passwords.

T060 passes for this scope, with evidence in `internal/provider/webhosting_resource_test.go` and `internal/provider/webhosting_live_test.go`.
T038/T041 remain partial across other services. T056 remains failed for the earlier console-present webmail; these four hosting cleanups
are not a blanket cleanup claim. Data-plane access, migration, other product/version variants and billing termination remain unverified.

## Hosting catalog data sources (T061, 2026-09-19)

Registered `iwinv_webhosting_products` and `iwinv_webhosting_servers` using the previously verified catalog decoders.
Product queries support omitted, SHARE and SINGLE filters. Server queries require an exact product ID; decimal server IDs remain
strings without float64 conversion. Both data sources sort by ID and publish complete typed results only. They do not select a
provisioning product/server automatically. Prices, VAT and storage/traffic units remain excluded.

Go 1.26.1/Terraform 1.14.2 live read-only acceptance passed in 18.40 seconds, covering the three product filters, a product-scoped
server lookup and a subsequent no-change plan. No service was created, changed or deleted. Logs remain private.
Synthetic Core tests passed IDs above 2^53, query preservation, literal display text, PHP-label sorting, empty optional strings,
empty arrays and rejection of invalid inputs before requests. Duplicate/malformed arrays, changed pagination metadata and API errors
fail both data sources rather than publish partial results. T061 passes for these catalog contracts; creation availability and
other product/version lifecycle coverage are separate. Evidence: `internal/provider/webhosting_catalog_data_source_test.go`.

## DBMS adapter and Terraform lifecycle (T062/T063, 2026-09-19)

Fresh catalog validation stopped the first adapter run before any create: the complete 113-row catalog contains four `available` rows
with empty-string product IDs. C30 records this alongside product IDs shared by versions. The adapter preserves empty-ID rows for
review, rejects null/missing IDs and blocks creation with an empty ID. It does not invent a version selector or silently deduplicate.
The corrected adapter run selected one available STD Redis product with an unambiguous nonempty ID and passed in 75.17 seconds.
Two new services reached active; omitted descriptions read as empty strings and literal descriptions round-tripped without HTML decoding.
A two-IP JSON update wholly replaced the first service's allowlist while preserving the peer. Both exact IDs were acknowledged deleted and absent.

Terraform 1.14.2 / Go 1.26.1 race-enabled acceptance passed in 180.69 seconds. Two resources coexisted and four total service identities
were created across fresh-account replacement and external-deletion recovery. In-place authoritative-set updates retained the ID;
external allowlist drift was restored. Full readable-attribute import, persisted re-import without initial account history and no-change
plans passed. Fresh-account create-before-destroy preserved the peer, and final deletion/absence was verified for all four identities.
Together the adapter and Terraform runs created six DBMS services and cleaned up all six. No pre-existing DBMS, database contents,
network attachment, DNS or message recipient was modified. Intents, receipts, identifiers, Terraform state and logs remain private.

Synthetic tests cover exact IDs above 2^53, separate create/list domain shapes, malformed/partial lists, failed-create ID recovery through
Core cleanup, unknown values, invalid/empty/CIDR/IPv6 allowlists, order-insensitive sets, update/delete failures and wait expiry retaining
state, hidden creation identity, conservative replacement guards and explicit replacement plans. Explicit same-account replacement is
planned only, not applied. The account name is creation history absent from Read; it is not inferred from the domain on import.

T062/T063 pass within the documented STD Redis control-plane scope. T037/T038/T039 remain partial across other products/services;
T056 remains failed for the earlier webmail. Data connectivity, backup/migration, engine variants, account reuse timing and billing
termination remain unverified. DBMS catalog lookup is internal, not a registered data source. See [design decisions](dbms-lifecycle.md).

## DBMS product data source (T064) — 2026-09-19

Registered `iwinv_db_instance_products` with optional exact `product_type` and `engine` API filters and a sorted list of full product rows.
Empty creation IDs and IDs repeated across versions are retained (C30); missing/null IDs, malformed/duplicate tuples, partial metadata,
unknown/invalid filters and API errors are rejected. No automatic product selection or creation-time version selector is implied.

`TestAccDBProducts` passed in 49.77 seconds under Terraform 1.14.2/Go race: unfiltered reads, all six engine filters, both tier filters,
the STD/redis combination and subsequent no-change plans. This is filter acceptance and metadata validation, not independent engine identity
or provisioning verification. Synthetic Core checks cover empty/repeated-ID preservation, sorting, errors and input rejection.
No cloud resources were created or modified. The DBMS console list independently appeared empty without a search filter after T062/T063 cleanup.
The development provider now has ten data sources and five managed resources. T038 overall, webmail T056 and Registry release remain incomplete.

## Cache adapter and busy rejection (T065) — 2026-09-19

Added typed adapters for all five cache control-plane operations, nullable product IDs and exact service IDs. C31 records the catalog SHARE
versus service SINGLE label discrepancy; neither is rewritten or used to promise isolation. Create sends verified JSON pw.FTP and excludes
ignored create-time referrers. Passwords do not serialize into typed input journals or readable models. This does not yet establish Terraform
write-only plan/state exclusion.

The first two-service live run passed in 89.81 seconds, including one busy PUT rejection with unchanged Read followed by accepted replacement.
A second two-service run removed the 45-second pre-delete fixture delay and passed in 45.49 seconds. Both PUT and DELETE received one exact
busy rejection each; the target remained unchanged and a bounded reconciled retry succeeded. The common client classifies only the exact
method/path/status/code/message/result combination and never exposes response text or automatically retries.

All four new cache IDs received delete acknowledgements and exact-ID absence; the first run also had an independently empty unfiltered console
list. Existing infrastructure was untouched. Core resource/import/drift/replacement/secret-state recovery, catalog data source, tenant APIs,
other product variants, data-plane access and billing remain pending. See [cache decisions](cache-lifecycle.md). T038 and webmail T056 stay open;
public development registration remains ten data sources and five resources.

## Cache Terraform lifecycle (T066) — 2026-09-19

Registered `iwinv_content_cache` with complete referrer-set ownership, write-only initial FTP password, a local password-version replacement
trigger, exact-ID import and conservative fresh-account replacement when clearing a nonempty set. Names/descriptions/products/account changes
also replace. Only precisely classified busy PUT/DELETE rejections can retry after the complete parent remains unchanged; no uncertain replay.

`TestAccContentCache` passed in 116.04 seconds on Terraform 1.14.2 with Go race: coexistence, empty/nonempty initialization, stable-ID updates,
external drift restoration, full readable import, persisted no-password/version import and no-op, create-before-destroy clear with fresh account,
version/description replacement and external deletion/recreation. Saved compressed plans/state exclude the generated ephemeral password.
One busy PUT was reconciled; busy DELETE uses separate T065 live evidence and synthetic Core tests, not a claim from this run.
Synthetic tests also cover failed setup cleanup, delayed/malformed creation, unknown replacement plans, taint/explicit replacement and error-state retention.
All five new identities have delete acknowledgements and exact-ID absence. A refreshed console independently showed an empty unfiltered cache list.
No pre-existing infrastructure was mutated. [Cache design](cache-lifecycle.md) and the bilingual resource guide document the destructive cases.

Development registration is now ten data sources and six managed resources. The cache catalog data source, other products, tenant/content APIs,
FTP access, billing, overall T038, webmail T056 and Registry release remain incomplete.

## Cache catalog (T067) — 2026-09-19

Registered `iwinv_content_cache_products`, preserving null/empty product IDs and sorting complete rows independently of API order.
Exact SHARE/SINGLE filters reject unknown-at-Read, empty and differently cased input. Missing/invalid IDs, duplicate identities,
partial metadata, malformed rows, filter mismatches and API errors fail the full Read. No creation choice is inferred from ordering.

`TestAccCacheProducts` passed in 13.37 seconds with Terraform 1.14.2 and Go race: unfiltered/SHARE/SINGLE reads, observed null ID and
no-change plans. Synthetic Core also distinguishes null from empty strings and tests changing API order. No services were created.
Development registration is now eleven data sources and six resources; other products, tenant/content APIs and Registry release remain open.

## NAS typed adapter and readiness (T068) — 2026-09-19

A scoped readiness probe observed pending then active at 7.12 seconds of polling, verified 100 GB/literal description/initial RO permissions,
replaced the map with RW/RO and deleted the fresh service with acknowledgement and exact-ID absence. It confirmed that sharename is not readable.
This is not a latency guarantee or permission to infer a share name from mount information.

`TestAccNASControlPlaneWrites` then passed in 25.82 seconds with Go race: two coexisting api_nas services at the catalog minimum of 100 GB,
active waits, literal/omitted descriptions, a complete two-host permission-map replacement, retained-host RW-to-RO change/removal of the other
host, peer preservation and two acknowledged deletions with exact-ID absence. Including the probe, all three new NAS IDs were cleaned up.
The independently loaded API NAS console at /na showed an empty list without a search filter; its guide link identifies api-nas.

Typed schemas preserve exact int64 IDs, catalog empty IDs/null versions/zero coming-soon bounds and opaque mount text. The update receipt is
an ip/acl object array, distinct from create/Read's IP-to-mode map. Synthetic tests cover partial-create ID retention, invalid inputs, read
completeness, permissions/duplicate acknowledgements, errors, no write retry and excluded credentials/creation history.
See [NAS lifecycle decisions](nas-lifecycle.md). NAS Terraform registration/import/replacement, NFS/files, tenant APIs, billing, overall T038
and the separate webmail cleanup failure T056 remain incomplete. Existing infrastructure was untouched.

## NAS Terraform Core acceptance (T069), 2026-09-19

`TestAccSharedStorage` passed in 82.29 seconds using Terraform 1.14.2 and Go race. Two fresh api_nas services coexisted at 100 GB.
The test verified full RO/RW map replacement with stable identity, external permission drift restoration, full readable-attribute import,
persisted import/no-change plan without `share_name`, and permission updates after that import. A fresh-share create-before-destroy
replacement changed capacity to 200 GB and preserved a literal Korean description; the next plan had no changes. External deletion
followed by a fresh-share recreation also passed. All four created IDs received deletion acknowledgements and validated exact-ID absence.
The independently refreshed API NAS console at /na showed an empty list with a blank search field. No pre-existing services were mutated.

Synthetic Core tests cover map ordering, unknown permission/capacity values, invalid input rejection, exact IDs above 2^53,
failed-create ID retention and cleanup, never-verified missing identities, concurrent-parent guards, read/update/delete failures,
wait timeouts and no write replay. Explicit taint/`-replace` paths are plan-only tests; same-share recreation is not claimed.
Capacity changes are destructive replacement, not resize or migration. Only permissions update in place. Import intentionally omits
unreadable share-name history; mount information remains opaque. NFS/file access, actual permission enforcement, tenant API, backups,
other products and billing termination remain unverified. Overall T038 and independent webmail cleanup T056 stay incomplete.

Registration is eleven data sources and seven managed resources. NAS product lookup remains an internal adapter. See the
[resource guide](../../docs/resources/shared_storage.md) and [lifecycle decisions](nas-lifecycle.md). No Registry release exists.

## NAS product data source (T070), 2026-09-19

`iwinv_shared_storage_products` exposes the complete catalog, sorted by ID/name, retaining coming-soon empty IDs,
nullable versions and zero capacity bounds. Minimum/maximum capacity uses the API's documented GB; this is not an in-place resize
capability or an independent version selector. No default product is selected and no undocumented filters are invented.

`TestAccStorageProducts` passed in 3.13 seconds with Terraform 1.14.2/Go race, including a no-change plan. It observed an available
api_nas row with 100–2000 GB bounds and unselectable rows with empty IDs, null versions and zero bounds. The run was read-only.
Synthetic Core checks reverse response ordering, distinguish null and empty versions, preserve literal names, accept empty arrays,
and reject API errors, missing/null/non-string IDs, missing/non-string versions, missing/negative/fractional/reversed bounds,
duplicate identities and changed metadata. Catalog visibility does not validate every product, pricing, files or billing.

Registration is now twelve data sources and seven resources. All five documented NAS control-plane operations have registered
coverage; separate tenant/NFS surfaces, overall T038, webmail cleanup T056 and Registry release remain incomplete.
[Catalog guide](../../docs/data-sources/shared_storage_products.md) · [NAS lifecycle](nas-lifecycle.md).

## Billing read contracts (C32/T071), 2026-09-19

Read-only probes verified integer KRW amounts, page-local `count`, string page/size metadata and size-one/size-two page agreement.
Date filters use bill date in the tested records, including a record with a different usage start; exact date/price boundaries include
matches. VAT-exclusive price filters are checked against full typed rows. Empty filtered queries return exact HTTP 400 `EMPTY_SET`,
whereas an offset after the end returns HTTP 200 and an empty array. Only first-page list EMPTY_SET is normalized; late errors fail.

The new internal `billing.Service` preserves exact signed int64 amounts and literal dates/names; it excludes payment instruments,
invoice and tax URLs. Synthetic tests cover precision above 2^53, malformed/null/overflow data, pagination metadata and duplicate pages,
iteration bounds, cancellation, invalid filters, privacy exclusions and no partial output on errors. No billing Terraform type is
registered yet. `TestAccBillingReads` passed in 18.07 seconds with Go race, including exact, one-sided and negative-bound filters; see [billing contracts](billing-contract.md).

Two listed historical bill IDs and the documented BILL-live detail ID returned HTTP 403 CHECK_IP (nested code 9), while current/list
reads succeeded with the same local credentials. Detail remains unavailable and no allowlist or account setting was changed.
Currency scaling is not invented; timezone, alternate currencies, refunds/credits, unpaid variants and nested detail need further evidence.
T042 is now in progress; T056 webmail cleanup and the full goal remain incomplete. No billing record or cloud resource was mutated.

## Billing Terraform data sources (T072), 2026-09-19

Registered `iwinv_current_bill` and `iwinv_bills`. The current estimate requires exactly one row and marks all fields sensitive;
the complete bill list and price filters are sensitive. Sensitive values remain in state/plans; payment instruments and invoice/tax
links are absent from the schema and artifacts. Unknown filters never trigger an unfiltered account read. Dates are validated before
requests, while observed date text and exact signed int64 money are preserved without timezone/currency conversion.

`TestAccBillingDataSources` passed in 23.68 seconds on Terraform 1.14.2/Go race, reading current/list/empty-filter results and checking
sensitive state plus a no-change plan during a stable observation. Synthetic Core verifies money above 2^53, plan/state sensitivity,
root-output rejection, saved-artifact privacy exclusions, stable multi-page ordering, empty/multiple current cardinality, errors,
late failures and unknown/zero/negative filters. No billing or cloud mutations occurred.

Development support is fourteen data sources and seven resources. Detail still returns CHECK_IP; T042 timezone/detail gaps and
independent webmail cleanup T056 remain open. A changing live estimate may change future plans; no price-freezing guarantee is made.
[Current estimate guide](../../docs/data-sources/current_bill.md) · [Bill list guide](../../docs/data-sources/bills.md).
