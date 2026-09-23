# Instance read contract preparation

[한국어](../ko/instance-read-contract.md) · [Index](../../README.en.md)

## Implementation boundary

`internal/services/compute/instances.go` adds internal list and exact-ID read methods. No Terraform instance resource or data source is registered. Existing public support remains 18 data sources and 7 resources. This prepares the Read boundary for P2; it does not bypass the blocked compute-zone create contract.

The [list API](https://iwinv.readme.io/reference/getv1instances) explicitly limits visibility to API-supported zones. On 2026-09-23, a newly created console test server was running in the ordinary KR1-Z03 zone, while the authenticated list for the same account returned HTTP 200/success with zero rows. An empty list therefore does not prove deletion or account-wide absence. Detail requests using the console's numeric selection ID and displayed UUID both returned `ID_INVALID`; this does not establish the detail contract for an API-supported-zone fixture. The [detail API](https://iwinv.readme.io/reference/getv1instancesinstanceid) returns a result array, so detail requires exactly one matching ID. Empty, multiple or mismatched results remain errors. Authentication, 404, throttling and server errors are propagated without converting them into absence or clearing state. Import and managed lifecycle are deferred until a populated API-owned fixture establishes the contract.

## Selected projection

The [official field table](https://api-kr.iwinv.kr/fields/v1/instances), checked 2026-09-19, defines the fixed mask **3599**:

| API field | Bit | Typed result |
| --- | --- | --- |
| instance_id | 1 | ID |
| name | 2 | Name, preserving empty string |
| description | 4 | Present string or null; no HTML/Unicode normalization |
| status | 8 | Nonempty raw status; no guessed readiness mapping |
| zone | 512 | zone_id |
| flavor | 1024 | flavor_id |
| image | 2048 | image_id |

`default_account` (128) and `vnc` (16384) are never requested. Even if unsolicited, those fields have no typed destination. The adapter exposes neither raw responses nor default passwords/console links. Other nested metadata is ignored. Missing/null selected identifiers or wrong shapes fail closed, rather than receiving guessed defaults. Populated validation could require revising these candidate rules before registration.

List reads request ten rows per page, verify count/page number/page size, reject repeated IDs across or within pages, and return a sorted complete result only after a short page. An exact full final page requires one more empty page. Pagination is capped at 1,000 pages and respects context cancellation; late errors discard the partial result. No total or snapshot token is documented, so this cannot promise a consistent account-wide snapshot during concurrent changes. Filters are not yet exposed.

## Evidence and next gates

Synthetic tests cover fixed query masks, unsolicited credential canaries, Unicode and null/empty handling, malformed selected fields, zero/exact/multiple detail matches, duplicate or invalid late pages, status failures, bounded traversal, cancellation and no replay after a late transport error. The tests use invented identifiers only.

On 2026-09-19, `TestAccInstanceListRead` passed against the official endpoint with **zero API-visible instances**. This confirms authentication and empty-page metadata for the selected mask only. It does not prove populated projection, field-mask enforcement on populated responses, detail, missing-ID behavior, pagination under load, or resource lifecycle. No cloud resource was created or changed. Private logs contain no public fixtures or credentials in this repository.

To repeat the bounded read with credentials already supplied privately:

```sh
TF_ACC=1 IWINV_LIVE_READ=1 go test -race ./internal/services/compute -run '^TestAccInstanceListRead$' -count=1 -v
```

Before registration, resolve Compute eligibility, create one owned fixture, compare requested fields with safe console observations, verify nullability and exact detail identity, measure pagination and confirmed absence, then implement CRUD/import/state recovery and bilingual user guides. T008 is in progress, not passed; C01/C06 and the P2 gates remain open.
