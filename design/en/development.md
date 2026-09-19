# Run the development provider

[한국어](../ko/development.md) · [Progress](contract-progress.md)

There is no Registry release. The local binary implements zone, image, instance-type and SSH-key data sources.
No managed resources or lifecycle operations are registered. Never apply the proposed instance examples yet.

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
- Full signing, publishing, state migration and managed-resource acceptance are later gates.

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
The test verifies live Go JSON POST/PUT/DELETE and the security-group behavior that ignores empty descriptions.
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
