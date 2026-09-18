# API contracts and unresolved behavior

[한국어](../ko/api-contract.md) · [Index](../../README.en.md) · Revision: 1

Evidence date: 2026-09-18. All facts below are **documented**, not authenticated live observations.
Public documentation and OAuth discovery were read. No account API, mutation, or billable test was executed.
The [inventory](../inventory/api.json) extracts endpoint facts from embedded OpenAPI definitions, not a vendor SDK.
It is not a validated unified OpenAPI specification or proof that all services are covered.

## Contract findings

| ID | Documented evidence | Design consequence and required verification |
| --- | --- | --- |
| C01 | Main API is `https://api-kr.iwinv.kr`; introduction retains Beta/Private Beta wording | Verify account enablement and supported zones before claiming existing-account compatibility |
| C02 | HMAC-SHA256 over timestamp concatenated with path; query omitted, no trailing slash; three X-iwinv headers | Test byte-exact canonicalization, path encoding, fresh signatures, ±5-minute clock window |
| C03 | 60 requests/minute; API keys use IP allowlists | Determine quota scope, 429/Retry-After behavior and runner egress requirements |
| C04 | Instance listing covers API-supported zones; pagination defaults include page size 10 | Page through all results; empty list cannot establish account-wide absence |
| C05 | Instance Create returns 202; example includes an ID and `building` | Confirm usable ID timing, wait states and failure persistence; List enum does not include `building` |
| C06 | Instance detail is a result array and requires selected `fields` for richer data | Validate result cardinality, bitmask, missing/null fields and nested types; never assume object result |
| C07 | Detail schema exposes nested zone/flavor/image and IP array, but no SSH/script history | Verify import reconstruction; creation response includes ssh_key/user_script_id but is not refresh evidence |
| C08 | Detail schema can contain default_account.password and VNC link | Request least fields; redact payloads; never persist these incidental values |
| C09 | Name/description have length limits and URL-encoding instructions; description appears null in reads | Test Korean NFC/NFD, bytes vs characters, empty clear, single vs double encoding |
| C10 | API batch creation supports up to 10 | Provider creates exactly one remote object per resource; reject unexpected result counts |
| C11 | Resize/rebuild and delete return 202 | Measure downtime, IP/disk effects, rollback, quotas, final states and billing termination separately |
| C12 | Shutdown is documented as power-switch-like; charges continue while powered off | Do not market shutdown as graceful OS stop or cost termination |
| C13 | No idempotency key found in reviewed create contract | Resolve uncertain create outcomes without blind retry; ask vendor about request tokens and reconciliation |
| C14 | Block storage creation requires instance_id/type/size and returns attached instance ID | Do not assume standalone EBS creation; prove detach, reattach, zone affinity and single ownership |
| C15 | Block storage GET and SG detail GET are documented with 202 | HTTP status alone cannot drive async logic; inspect actual payload semantics per endpoint |
| C16 | SG rule description uses IN/OUT and TCP; example uses inbound/tcp; IDs mix integer/string | Verify accepted values, returned canonical form, rule identity, ports, CIDR, ICMP and duplicate behavior |
| C17 | Instance creation security_group, attach_public_ip and block_storage.N.type are deprecated | Prefer dedicated APIs only when verified; do not expose deprecated controls as stable promises |
| C18 | User scripts only have list API and console-only creation; SSH keys only list in reviewed docs/CLI | Initial lookup/reference only; keep missing create/update/delete as explicit vendor gaps |
| C19 | Five non-IaaS service families have 26 operations without response schemas in the extracted docs | Capture redacted live structures before defining resource IDs, polling, import, or data sources |
| C20 | Some service docs advertise multipart Content-Type but requestBody says application/json | Verify actual transport per operation; do not generate an HTTP client blindly |
| C21 | NAS/DBMS PUT allowip accepts an array; no per-IP deletion documented | Determine add vs replace, empty set and read-back; choose authoritative-set ownership if appropriate |
| C22 | Cache referrer PUT has no usable body fields in extracted schema | Block referrer management until body, clear behavior and read-back are known |
| C23 | Webmail subaccount has create/delete but no separate list/detail in index | Determine parent response visibility; no managed account release without authoritative Read/import |
| C24 | Object CLI lists/copies/moves/removes objects; presign docs specify 15 minutes | Separate S3/data-plane credentials and endpoint compatibility; no assumption of full AWS S3 feature coverage |
| C25 | SMS and Alimtalk use different domains/headers from control-plane HMAC | Use service-specific auth/error/rate adapters; send operations are non-idempotent Action candidates |
| C26 | Alimtalk template changes are restricted by review status | Design state-dependent updates/replacement and review timeouts; validate all template states |
| C27 | MCP supports OAuth and mcp:tools; authenticated tool list not inspected | Use future tools/list comparison for gap discovery, not a Provider runtime dependency |
| C28 | Fields page says OpenStack, but documented auth is proprietary HMAC | Verify whether standard Keystone/service endpoints are customer-accessible before claiming OpenStack interoperability |

Sources: [API request](https://iwinv-common.readme.io/reference/api-request),
[response](https://iwinv-common.readme.io/reference/api-response),
[key management](https://docs.iwinv.kr/developers/api/api-key-management/),
[instance create](https://iwinv.readme.io/reference/postv1instances),
[instance detail](https://iwinv.readme.io/reference/getv1instancesinstanceid),
[fields](https://api-kr.iwinv.kr/fields/v1/instances),
[block storage create](https://iwinv.readme.io/reference/postv1blockstorages),
[SG rule create](https://iwinv.readme.io/reference/post_v1-security-groups-id-rules).
Other service and CLI sources are attached to each entry in the inventory and [source register](../sources.md).

## Evidence needed before implementation stabilizes

For every operation record: source date, method/path, credential family, request content type, field types,
constraints, defaults, HTTP and business errors, response ID, completion condition, pagination, consistency,
idempotency, rate scope, billing effects, supported products/zones and deprecations.
Store only reviewed synthetic/redacted fixtures in Git. Raw account captures remain outside this repository.
Record contract fingerprints/diffs on refresh and investigate new, removed, or changed endpoints.

The inventory's `response_schema_present` is merely a documentation flag. Even present schemas contain
null-only properties and inconsistent types; use live evidence to decide optional/nullable decoding.
Do not normalize unrecognized business success codes into success.

## Vendor questions to resolve

1. Is the API generally available, and which existing products/zones/accounts are supported?
2. Are versioning/deprecation guarantees, a canonical complete OpenAPI file, and a test environment available?
3. Are create idempotency tokens, request IDs, operation-status APIs and uncertain-outcome recovery supported?
4. Can SSH keys and scripts be managed and read back? Can webmail accounts be listed/read?
5. What are the actual write encodings, authoritative allowlist semantics, and scope of the 60/minute quota?
6. Can block storage exist independently, and how do instance deletion, disk retention and billing interact?
7. Are additional official operations available through CLI/MCP but absent from public API docs?
8. Are customer-accessible standard OpenStack endpoints available?

These are prepared questions, not messages sent to the vendor. Each answer must update the associated Cxx
finding, capability entry, schema decision and verification gate.
