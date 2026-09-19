# Run the development provider

[한국어](../ko/development.md) · [Progress](contract-progress.md)

There is no Registry release. The local binary implements only `iwinv_availability_zones`.
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
