---
page_title: "iwinv_shared_storage Resource - iwinv"
subcategory: "Storage"
description: |-
  Manages an API NAS service and its authoritative IPv4 permission map.
---

# iwinv_shared_storage (Resource)

[한국어](../ko/resources/shared_storage.md) · [Development installation](../../design/en/development.md)

Development control-plane resource for API NAS. No Registry release yet. This resource manages service configuration;
it does not mount storage, access files, execute mount information, manage tenant credentials or migrate data.

## Example

Review the available product and use a fresh share name. The [complete example](../../examples/resources/iwinv_shared_storage/main.tf)
uses documentation-only IPs; replace them with reviewed client IPv4 addresses.

```hcl
resource "iwinv_shared_storage" "example" {
  product_id  = var.product_id
  share_name  = var.share_name
  name        = "tf-example-storage"
  description = "Shared application files"
  size_gb     = 100
  allowed_ips = {
    "192.0.2.10" = "RW"
    "192.0.2.11" = "RO"
  }

  lifecycle { prevent_destroy = true }
}
```

## Schema and ownership

| Attribute | Type | Behavior |
| --- | --- | --- |
| `product_id` | required string | Exact nonempty creation ID; changes replace. Catalog visibility does not guarantee eligibility. |
| `share_name` | optional string | Initial 6–20 ASCII letters/digits; required to create. Unreadable creation history, omitted on import. Adding/changing/removing it replaces. |
| `name` | required string | Literal alias, 4–32 Unicode code points; changes replace. |
| `description` | optional + computed string | Default empty, maximum 50 code points. Blank omitted on create; changes replace. |
| `size_gb` | required int64 | Documented 100–2000 GB range; changes replace the entire service, not resize. |
| `allowed_ips` | required map(string) | Complete nonempty map of canonical IPv4 hosts to exact `RO` or `RW`; updates in place. |
| `id` | computed string | Exact positive decimal `service_idx`; import ID. |
| `status` | computed string | Observed control-plane status. Active does not prove file access or permission enforcement. |
| `address` | computed string | Observed domain; no inferred scheme, port or DNS ownership. |
| `mount_info` | computed string | Opaque observed text; never executed or used to reconstruct share-name history. |

Local `timeouts` default to create/update/delete `5m`, read `1m`; durations must be positive.
Timeout-only updates make no remote write. Names and descriptions preserve literal Korean and HTML-looking text.

One resource owns the **entire permission map**. A PUT removes omitted hosts and changes retained-host permissions.
Do not split individual memberships across resources/states. Empty maps, CIDR, IPv6, null permissions and alternate role spellings
are rejected. External changes are drift; Terraform restores configured permissions. Read-back verifies API configuration, not NFS behavior.

## Replacement and import

Only permissions update in place. Product, share-name history, name, description and capacity changes replace and delete storage.
Replacement requires a known fresh share name. NAS share-name reuse rules remain unverified; no hosting/cache reuse interval is assumed.
`create_before_destroy` can establish the new share first, but **does not copy data or reconfigure clients**.
The example's `prevent_destroy` blocks destruction until explicitly removed. Back up and plan migration before replacement.
Explicit taint/`-replace` can bypass ordinary attribute comparisons; supply a fresh name before applying these paths.

```sh
terraform import iwinv_shared_storage.example 123456789
terraform plan
```

The ID is synthetic. Match the actual product, name, description, size and permission map; omit `share_name` because Read cannot recover it.
Import restores all readable attributes and allows a no-change plan and permission updates without creation history.
Keeping a share name in imported configuration intentionally requests replacement. One service must belong to one Terraform state.

## Failure recovery

Create sends one POST, stores its exact returned ID before checking the remaining receipt, then waits for active status and matching
readable settings. Failed or delayed initialization retains the ID. A never-verified missing service produces a Read diagnostic;
a previously active service can leave state after validated exact-ID absence. Malformed lists, duplicate IDs and HTTP errors never mean absence.

Before updating permissions, the provider reads the exact parent and checks that its readable settings still match prior state.
Concurrent changes, missing parents or failed reads stop the write. Failed updates retain prior state for reconciliation.
Delete requires a successful acknowledgement and validated absence. No NAS write is automatically retried, including busy or transport errors.
Refresh and reconcile an uncertain outcome before another write; inspect retained failed-create state before Core replacement.
API cleanup does not independently establish billing termination.

See [lifecycle decisions](../../design/en/nas-lifecycle.md) and [verification evidence](../../design/en/contract-progress.md).
Sources: [create](https://iwinv-api-nas.readme.io/reference/공유-스토리지-생성),
[permissions](https://iwinv-api-nas.readme.io/reference/접근-허용-ip-추가),
[delete](https://iwinv-api-nas.readme.io/reference/공유-스토리지-삭제).
