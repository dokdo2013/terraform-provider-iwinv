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
The hosting server catalog and user-script list returned 404. An empty successful list and a 404 cannot be normalized indiscriminately.
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
