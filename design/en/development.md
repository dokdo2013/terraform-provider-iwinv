# Run the development provider

[한국어](../ko/development.md) · [Progress](contract-progress.md)

There is no Registry release. The local binary implements fourteen zone/image/instance-type/SSH-key/hosting/DBMS/cache/NAS-catalog/billing data sources
and seven [NAS](../../docs/resources/shared_storage.md), [cache](../../docs/resources/content_cache.md), [DBMS](../../docs/resources/db_instance.md), [webhosting](../../docs/resources/webhosting.md), [security-group](../../docs/resources/security_group.md) / [rule resources](../../docs/guides/security_group_rules.md). Proposed instance examples are not yet runnable.

## Build and verify

Requires Go >=1.25.8 and Terraform >=1.14.0. The dependency versions are pinned in [go.mod](../../go.mod).

```sh
go build -o bin/terraform-provider-iwinv .
go test -race ./...
go vet ./...
python3 scripts/check_docs.py
IWINV_PROTOCOL_TEST=1 go test ./internal/provider -run TestProtocol -v
```

The protocol tests use synthetic API fixtures. If Terraform is not discoverable, set
`TF_ACC_TERRAFORM_PATH` to the absolute path of the Terraform executable.

Create a **separate local** Terraform CLI configuration file, replacing the path with the absolute
directory containing the built binary. Do not overwrite an existing personal CLI configuration.

```hcl
provider_installation {
  dev_overrides {
    "dokdo2013/iwinv" = "/absolute/path/to/terraform-provider-iwinv/bin"
  }
  direct {}
}
```

Point `TF_CLI_CONFIG_FILE` to that file. With development overrides, skip `terraform init` for this
example: the provider is not published and Registry installation would fail.

```sh
terraform -chdir=examples/data-sources/iwinv_availability_zones validate
terraform -chdir=examples/data-sources/iwinv_availability_zones plan
```

Validation does not require credentials. Plan needs the control-plane keys from
`IWINV_ACCESS_KEY` and `IWINV_SECRET_KEY`, injected through your secret manager/environment.
Do not put real keys in examples, command history, `.tfvars`, GitHub issues or committed files.
Provider attributes `access_key` and `secret_key` override the respective environment variable;
an explicit empty or unknown value does not silently select environment credentials.

## Availability-zone data source

```hcl
data "iwinv_availability_zones" "available" {}

output "zone_ids" {
  value = data.iwinv_availability_zones.available.zone_ids
}
```

| Attribute | Type | Meaning |
| --- | --- | --- |
| `zone_ids` | list(string), computed | Exact API IDs, sorted lexically |
| `names` | list(string), computed | Display names in the same order |
| `zones` | list(object), computed | `id`, `name`, `status` for each zone |

No arguments, region default, ID normalization or status filtering is invented. The data source
fails on malformed rows, inconsistent counts and duplicate IDs. An empty array is a valid empty catalog,
but an API error never becomes an empty catalog. Import does not apply to data sources.
Ownership is read-only; this does not manage zones or prove that all console servers are API-visible.
The shared [example](../../examples/data-sources/iwinv_availability_zones/main.tf) is the executable source for both languages.

## Explicit live read-only acceptance

```sh
TF_ACC=1 IWINV_LIVE_READ=1 go test ./internal/provider -run '^TestAccAvailabilityZones$' -v
```

This test reads the zone API through Terraform, checks nonempty results and a subsequent no-change plan.
It creates no cloud resources. CI never sets this opt-in or receives cloud credentials.
Provider alias isolation is not established by plugin-testing's reattached aliases; its documented
separate factory mechanism is used for synthetic client isolation tests. The native binary separately passed a real-key primary read while only the synthetic-invalid alias was rejected. This is not a comparison of two valid customer accounts.

## Known limits

- A controlled multipart instance creation experiment returned HTTP 500 / `DEV_CHECK_RETURN`, without an instance ID.
  No automatic retry was made. Immediate and delayed API inventory checks were empty; this is not evidence of successful lifecycle or billing verification.
- Service-specific credentials for Object Storage/NAS/Cache/messaging and authenticated MCP OAuth discovery remain outstanding.
- Full signing, publishing, state migration and acceptance of the remaining managed resources are later gates.

After building, repeat the native binary alias check with:

```sh
IWINV_LIVE_READ=1 python3 scripts/test_alias_isolation.py
```

## Image and instance-type catalogs

The [catalog example](../../examples/data-sources/iwinv_catalogs/main.tf) uses the same development override and environment credentials.
`iwinv_images` and `iwinv_instance_types` expose `ids`, a lexically sorted list of exact API IDs.
They traverse every page; no names, prices or availability are inferred from list position.
Choose an ID deliberately, then set `image_id` or `instance_type_id` in the example to read its detail.

| Data source | Required input | Computed output |
| --- | --- | --- |
| `iwinv_image` | `id`: exact image ID | `visibility`, `image_type`: exact API strings |
| `iwinv_instance_type` | `id`: exact flavor ID | `name`: product display name |

Dots in flavor IDs are preserved. There are no implicit filters, latest-image selection or default product choices.
A missing ID, multiple results, or a returned ID differing from the requested ID is an error.
These are read-only catalog data sources: they do not own objects and import does not apply.
Catalog membership does not establish zone availability, image/product compatibility, capacity or current billing prices.
Public image detail is live-tested; private-image variants and richer product specifications remain unverified.

Pagination uses the observed page size of 10, validates page/count metadata, rejects duplicate IDs and changed product totals,
and stops at a short image page or the exact product total. A full image page requires reading the next page.
After 1,000 pages it fails explicitly. A late error returns no partial catalog. There is no API snapshot token,
so concurrent catalog changes cannot be fully excluded even with these checks. Reads are not automatically retried.

```sh
TF_ACC=1 IWINV_LIVE_READ=1 go test ./internal/provider -run '^TestAccCatalogs$' -v
```

This read-only acceptance selects the first sorted IDs solely as test inputs, reads both details,
and checks a subsequent empty plan. It is not an image or product selection recommendation.

## Shared write-client verification

The internal Go client supports JSON POST/PUT, multipart POST/PUT, and bodyless DELETE.
Requests are capped at 1 MiB; invalid paths, field names and payloads fail before network I/O.
Multipart values are transmitted verbatim; endpoint-specific percent encoding belongs in the verified service layer.
API errors, redirects and connection loss do not trigger automatic retries.
Successful HTTP 202 is preserved without claiming asynchronous completion or deletion.

Inspection of the Go 1.26.1 transport source identified transparent replay paths.
The client uses fresh HTTP/1 connections, disabling HTTP/2 stream replay and HTTP/1 retries on reused connections.
TLS verification remains enabled, at the cost of additional connections. Transport optimization needs a proven idempotency/explicit retry contract.
[Go HTTP/1 transport](https://cs.opensource.google/go/go/+/refs/tags/go1.26.1:src/net/http/transport.go),
[Go HTTP/2 transport](https://cs.opensource.google/go/go/+/refs/tags/go1.26.1:src/net/http/h2_bundle.go).

The following test is **not read-only**: it creates, updates and deletes one unattached security group.
Run only against an explicitly scoped test account and select a private journal directory outside the repository.
The directory requires mode 0700; files use 0600 and retain the create response/ID and deletion verification.
Cleanup targets only the ID returned by this run's create, including after test failure. Forced process termination requires journal-based manual recovery.

```sh
IWINV_LIVE_WRITE=1 IWINV_TEST_JOURNAL_DIR=/absolute/private/test-journals \
  go test ./internal/client -run '^TestAccControlPlaneWrites$' -v -count=1
```

Supply keys through the existing `IWINV_ACCESS_KEY`/`IWINV_SECRET_KEY` environment variables. CI never enables this gate.
The test verifies live Go JSON POST/PUT/DELETE through the typed network adapter, paginated inventory and exact-ID reads, single-pass description decoding, omitted-description updates, rejection of unsupported empty clears, and cleanup.
Earlier raw ASCII empty-update observations must not be generalized to escaped descriptions; see the latest [contract findings](contract-progress.md).
It does not verify a successful multipart lifecycle or a Terraform managed resource.

## Existing SSH key references

The [SSH key example](../../examples/data-sources/iwinv_ssh_keys/main.tf) uses the same development override and environment credentials.
It lists references to keys already registered with iwinv; it never generates, uploads, rotates or deletes a key.
Select the intended exact ID for `ssh_key_id`. Duplicate names are allowed and are not selectors.

| Data source | Input | Output |
| --- | --- | --- |
| `iwinv_ssh_keys` | None | `ids`: list(string); `keys`: list(object) with `id` and `name`, in the same lexical ID order |
| `iwinv_ssh_key` | Required `id`, exact API `ssh_key_id` | Computed `name` |

The API only documents a list operation, so even singular lookup traverses and validates all pages.
A missing ID is an error. Empty lists remain empty lists; errors never become empty successful state.
Private/public key material and creation timestamps are not output attributes. Returned reference IDs/names are stored in Terraform state.
Ownership is read-only; import does not apply. These data sources do not establish server-side SSH installation or account-wide visibility.
The reviewed public API has no key creation/deletion operation: that gap remains in C18.

Pagination uses 10 entries per request, requires consistent count/page metadata, rejects duplicates and unexpected total/page fields,
and reads another page after a full page. It stops on a short page and fails after 1,000 pages; late failures return no partial results.
The API does not offer a snapshot token, so concurrent key changes can still affect pagination. Reads are not automatically retried.

```sh
TF_ACC=1 IWINV_LIVE_READ=1 go test ./internal/provider -run '^TestAccSSHKeys$' -v
```

This live test requires at least one pre-existing key. It reads both data sources and verifies a subsequent empty plan.
Its first sorted ID is only a test input, not a recommended key-selection policy. No cloud objects are created or modified.
Source: [official SSH key list](https://iwinv-common.readme.io/reference/get_new-endpoint-1-1).

## Security-group attributes

The first managed resource is [iwinv_security_group](../../docs/resources/security_group.md).
Follow the development override above, then validate/review/apply the [resource example](../../examples/resources/iwinv_security_group/main.tf).
Unlike the data-source examples, apply creates a cloud object. Run destroy when finished and verify absence.
Do not use existing shared groups as test fixtures. The group resource does not manage inline rules. Attachments, empty group-description creation/clearing and attached-group deletion remain unsupported or unverified.

```sh
TF_ACC=1 IWINV_LIVE_TERRAFORM_WRITE=1 \
  IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  go test ./internal/provider -run '^TestAccSecurityGroup$' -v -timeout 12m
```

This separate live gate requires the same environment credentials. Do not enable it in public CI.
The wrapper inventories existing group IDs before writes and only permits mutations of new IDs returned by this execution.
A private mode-0600 journal atomically records request intent, create receipts, known IDs, delete attempts and verified absence.
Test stdout/state and journals can contain account identifiers; keep all of them outside Git. Never share raw logs as public evidence.
Fallback cleanup reads each owned ID even after a test failure and does not blindly replay an earlier DELETE.
An unidentified create or unverified deletion requires reconciliation using the journal; an empty test output is not cleanup proof.

The test covers supported attributes, stable-ID update, import with full attribute verification,
explicit state removal without remote destroy, re-import into persisted state followed by an empty plan,
external drift/repair, external deletion/recreation, and final destroy. Only the run-owned groups are involved.
Synthetic tests separately cover defaults, timeout-only changes, invalid/unknown inputs, delayed visibility,
API errors, waiter expiry and failed-create ID preservation through Terraform Core cleanup.
The imported-state test explicitly removes prior Terraform ownership before re-importing; importing over an existing address is invalid.

## Independent security rules

See the [ingress guide](../../docs/resources/security_group_ingress_rule.md), [egress guide](../../docs/resources/security_group_egress_rule.md),
and shared [lifecycle/recovery guide](../../docs/guides/security_group_rules.md). Their complete examples use parent references.
Schema validation, no-op plans, drift, replacements and dependency-ordered destroy run in synthetic CI without cloud credentials.

The lower-level rule adapter has a separate opt-in contract test:

```sh
IWINV_LIVE_RULE_WRITE=1 IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  go test ./internal/client -run '^TestAccRuleControlPlaneWrites$' -v -timeout 8m
```

Use `TF_ACC=1 IWINV_LIVE_TERRAFORM_RULE_WRITE=1` with `TestAccSecurityRules` and `TestAccSecurityEgressRule` for full Terraform lifecycle tests; the shared guide gives the command.
The adapter test creates one parent/two rules; the two Terraform tests together create three parents/eight rule identities including replacements.
Each test persists private receipts/IDs and verifies rule absence before removing parents. Neither uses pre-existing groups or server attachments.
Successful control-plane tests do not validate traffic filtering or resolve the blocked compute zone.

## Internal hosting adapter contract test

The [hosting resource](../../docs/resources/webhosting.md) is registered; [product](../../docs/data-sources/webhosting_products.md)/[server](../../docs/data-sources/webhosting_servers.md) catalog data sources are also registered (T061).
The lower-level adapter test is separate from Terraform lifecycle acceptance.
This opt-in test creates and deletes two new hosting services. It can incur costs and requires an authorized account,
private credential environment variables and a mode-0700 journal directory outside the repository.

```sh
IWINV_LIVE_WEBHOSTING_WRITE=1 IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  go test -race ./internal/client -run '^TestAccWebhostingControlPlaneWrites$' -v -count=1 -timeout 10m
```

The test fails before creation if no SHARE product supporting custom domains or PHP 8.4 server choice is available.
It uses distinct temporary account names and `.invalid` domains, without uploading data or changing DNS.
A private journal records intent before creation, receipts/IDs, deletion attempts, acknowledgements and list absence.
Inspect it after failure; do not replay uncertain creates. Account name reuse has a documented 24-hour restriction.
This test does not establish Terraform import/state behavior, data connectivity or billing termination. CI does not enable it.

## Terraform hosting acceptance

Use the [resource guide](../../docs/resources/webhosting.md) and [lifecycle decisions](webhosting-lifecycle.md).
Synthetic Core tests run with `IWINV_PROTOCOL_TEST=1` and `-run TestProtocolWebhosting`, without live credentials.
The separate live test creates four account identities, maintains two services concurrently, and tests fresh-account replacement,
import, no-change plans, persisted re-import, external deletion and cleanup. It never mutates pre-existing services.

```sh
TF_ACC=1 IWINV_LIVE_TERRAFORM_HOSTING_WRITE=1 \
  IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  TF_ACC_TERRAFORM_PATH=/absolute/path/to/terraform \
  go test -race ./internal/provider -run '^TestAccWebhosting$' -v -count=1 -timeout 12m
```

Provide authorized credentials only through the child process environment. Keep logs, state and journals outside Git.
The test requires a SHARE product with custom domains and PHP 8.4, uses random distinct account names and ephemeral initial passwords,
and records intent before each write. Acknowledged deletion and exact-ID absence must both be verified for every owned ID.
Fallback cleanup does not blindly replay an uncertain deletion. Inspect private evidence after any failure.
No DNS/content migration or billing termination is asserted. CI never enables this test.

## Hosting catalog reads

Use [`iwinv_webhosting_products`](../../docs/data-sources/webhosting_products.md) and
[`iwinv_webhosting_servers`](../../docs/data-sources/webhosting_servers.md) to review explicit creation choices.
The [catalog example](../../examples/data-sources/iwinv_webhosting_catalogs/main.tf) reads only and accepts a chosen product ID.
`IWINV_PROTOCOL_TEST=1 go test ./internal/provider -run TestProtocolHostingCatalog` uses synthetic fixtures.
`TF_ACC=1 IWINV_LIVE_READ=1 go test -race ./internal/provider -run '^TestAccHostingCatalogs$' -count=1`
performs authenticated reads and a no-change plan, without creating services. Use the private environment/log controls above.
Do not confuse catalog visibility with readiness or successful creation.

## Cloud DBMS lifecycle

See the [DBMS resource guide](../../docs/resources/db_instance.md) and [design decisions](dbms-lifecycle.md).
Synthetic Core tests use `IWINV_PROTOCOL_TEST=1 go test ./internal/provider -run TestProtocolDBInstance` without cloud credentials.
The adapter and Terraform live tests are separate opt-in runs:

```sh
IWINV_LIVE_DBMS_WRITE=1 IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  go test -race ./internal/client -run '^TestAccDBMSControlPlaneWrites$' -v -count=1 -timeout 12m

TF_ACC=1 IWINV_LIVE_TERRAFORM_DBMS_WRITE=1 \
  IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  TF_ACC_TERRAFORM_PATH=/absolute/path/to/terraform \
  go test -race ./internal/provider -run '^TestAccDBInstance$' -v -count=1 -timeout 15m
```

These create two and four new STD Redis service identities respectively, and may incur costs. Use an authorized account and the private
credential/log controls above. Every write intent and returned creation identity is journaled; existing IDs are excluded from mutation.
Each deletion requires an acknowledgement and exact-ID absence. Fallback cleanup never blindly repeats an uncertain deletion.
No SQL/Redis client query, data write, DNS change or billing termination verification is performed. CI never enables paid tests.

## DBMS product reads

`iwinv_db_instance_products` ([English](../../docs/data-sources/db_instance_products.md), [한국어](../../docs/ko/data-sources/db_instance_products.md))
uses `IWINV_LIVE_READ=1 TF_ACC=1 go test -race ./internal/provider -run '^TestAccDBProducts$' -count=1` for read-only acceptance.
T064 validates all documented filters, empty/version-shared IDs and a no-change plan. No new service is created.

## Internal cache adapter acceptance

The adapter test is a paid, explicit opt-in and creates two fresh `cache_lite` services. Use the private credential/log/journal workflow above.
`IWINV_LIVE_CACHE_WRITE=1 IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory go test -race ./internal/client -run '^TestAccCacheControlPlaneWrites$' -v -count=1 -timeout 12m`
T065 records every attempted write and only retries precisely classified busy rejections after an unchanged exact-ID Read. Generic errors,
transport uncertainty and accepted writes are never automatically repeated. Deletion acknowledgement and exact-ID absence are required for
all owned IDs. This adapter test is separate from the Terraform resource acceptance below and does not exercise tenant content APIs.

## Cache Terraform acceptance

See the [resource guide](../../docs/resources/content_cache.md) and [lifecycle decisions](cache-lifecycle.md).
`IWINV_PROTOCOL_TEST=1 go test -race ./internal/provider -run TestProtocolContentCache` runs synthetic Core tests without cloud writes.
T066 live acceptance creates five new `cache_lite` identities, including replacements, and incurs costs:

```sh
TF_ACC=1 IWINV_LIVE_TERRAFORM_CACHE_WRITE=1 \
  IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  TF_ACC_TERRAFORM_PATH=/absolute/path/to/terraform \
  go test -race ./internal/provider -run '^TestAccContentCache$' -v -count=1 -timeout 15m
```

Use the private credentials/log workflow above. The test records owned intents and IDs without passwords, excludes baseline IDs from writes,
scans saved plans/state for ephemeral passwords, and requires acknowledged deletion plus exact-ID absence for every created identity.
Fallback cleanup uses a fresh deadline and repeats only a precisely classified busy rejection after reconciliation, never accepted/uncertain writes.
CI does not enable this paid gate. This does not test tenant content APIs, FTP, billing or other products.

## Cache products and internal NAS adapter

T067: `TF_ACC=1 IWINV_LIVE_READ=1 go test -race ./internal/provider -run '^TestAccCacheProducts$' -count=1`
reads all cache catalogs and verifies no-change plans without mutations. See the [catalog guide](../../docs/data-sources/content_cache_products.md).
T068 uses a separate paid gate with two fresh 100 GB api_nas services:

```sh
IWINV_LIVE_NAS_WRITE=1 IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  go test -race ./internal/client -run '^TestAccNASControlPlaneWrites$' -v -count=1 -timeout 12m
```

Use the private credential/log workflow above. Every intended create/update/delete is journaled, baseline IDs cannot be mutated, and
an independent cleanup deadline requires acknowledged deletion plus exact-ID absence. Uncertain writes are not repeated. The test
does not mount storage, access files, authenticate to tenant APIs or verify billing. See [NAS decisions](nas-lifecycle.md).
The NAS resource and catalog data source have separate Core acceptance below. CI enables neither paid gates nor live credentials.

## NAS Terraform acceptance

T069 requires a separate paid gate. It creates four fresh identities through a 100 GB lifecycle and a 200 GB replacement,
restricts writes to a private ownership journal, and requires acknowledged deletion plus exact-ID absence for all fixtures.

```sh
TF_ACC=1 IWINV_LIVE_TERRAFORM_NAS_WRITE=1 \
  IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  TF_ACC_TERRAFORM_PATH=/absolute/path/to/terraform \
  go test -race ./internal/provider -run '^TestAccSharedStorage$' -v -count=1 -timeout 15m
```

Inject temporary HMAC credentials privately and keep all logs/journals/state outside this repository. This gate verifies only API NAS
control-plane behavior, including import without share history and capacity replacement; it does not mount or migrate files.
Use `IWINV_PROTOCOL_TEST=1` for synthetic `TestProtocolSharedStorage` coverage without live credentials or paid resources.

## NAS catalog acceptance

T070 is read-only: `TF_ACC=1 IWINV_LIVE_READ=1 go test -race ./internal/provider -run '^TestAccStorageProducts$' -v -count=1`.
Set the private credential environment and `TF_ACC_TERRAFORM_PATH` as above. It checks complete catalog output, empty-ID/null-version
rows, documented capacity bounds and a no-change plan. No paid resources or tenant API operations are created.
The offline `TestProtocolStorageProducts` suite uses `IWINV_PROTOCOL_TEST=1` and synthetic responses.

## Internal billing read acceptance

T071: `TF_ACC=1 IWINV_LIVE_READ=1 go test -race ./internal/services/billing -run '^TestAccBillingReads$' -v -count=1`.
Inject HMAC credentials privately and keep logs outside the repository. This test requires existing multi-page bill history;
it does not create invoices, perform payment or alter services. This adapter gate is separate from the Terraform Core acceptance below.
Detail access and timezone remain unresolved; see [billing contracts](billing-contract.md).

## Billing Terraform acceptance

T072: `TF_ACC=1 IWINV_LIVE_READ=1 go test -race ./internal/provider -run '^TestAccBillingDataSources$' -v -count=1`.
Use the private credential environment and `TF_ACC_TERRAFORM_PATH`. Keep financial logs/state/plans outside the repository.
This reads current/list/empty-filter results and checks sensitivity and a stable-window no-change plan; estimates can change later.
Synthetic `TestProtocolBilling` tests run with `IWINV_PROTOCOL_TEST=1` and inspect sensitivity, precision and excluded fields in saved artifacts.
