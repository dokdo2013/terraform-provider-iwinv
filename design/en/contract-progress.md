# P1 contract verification progress

[한국어](../ko/contract-progress.md) · [Verification](verification.md)

Observed 2026-09-19 UTC. This is an implementation checkpoint for [issue #1](https://github.com/dokdo2013/terraform-provider-iwinv/issues/1),
not provider availability or completion of P1. The client currently supports a single GET attempt.

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
Client isolation is only mock evidence for T011. Redirect/error redaction tests satisfy the
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

Use Go 1.25.8 as the module minimum, with CI on 1.25.8 and 1.26.1. The initial client has no external dependencies.
Local contract tests currently run on Go 1.26.1, macOS arm64. Terraform 1.14.2 is installed;
no provider CLI acceptance has run yet.

For P2, select Framework 1.19.0 and plugin-testing 1.16.0, matching the inspected
[official scaffolding dependency baseline](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/go.mod).
These dependencies will be pinned and compiled when the provider is introduced, not claimed tested now.
Adopt Terraform >=1.14 for the initial provider compatibility target and test the minimum separately before release.
[Ephemeral resources](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources)
require Terraform >=1.10 and [write-only arguments](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments)
require >=1.11; those feature floors do not lower this project's intended baseline.
Actions require their own acceptance and state-reconciliation tests before publication.
T013 remains in progress until the provider/protocol matrix is exercised.

Sources: [iwinv signing](https://iwinv-common.readme.io/reference/api-request),
[response envelope](https://iwinv-common.readme.io/reference/api-response),
[zones](https://iwinv.readme.io/reference/getv1zones),
[flavors](https://iwinv.readme.io/reference/getv1flavors),
[images](https://iwinv.readme.io/reference/getv1images).
