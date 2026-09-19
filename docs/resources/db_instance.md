---
page_title: "iwinv_db_instance Resource - iwinv"
subcategory: "Database"
description: |-
  Manages a cloud DBMS service and its authoritative IPv4 allowlist.
---

# iwinv_db_instance (Resource)

[한국어](../ko/resources/db_instance.md) · [Development installation](../../design/en/development.md)

Development control-plane resource; no Registry release yet. The verified service family is an available STD Redis product.
It manages service creation/deletion and the complete allowed-IP set. It does not manage database content, users/passwords,
backups, replication, DNS, SQL/Redis queries, engine upgrades or data migration. Other engine/product variants require acceptance.

## Example and ownership

Use the [complete example](../../examples/resources/iwinv_db_instance/main.tf) with a reviewed product ID, fresh account name
and your intended nonempty IPv4 host set. The example enables `prevent_destroy` because service deletion is irreversible.

```hcl
resource "iwinv_db_instance" "example" {
  product_id   = var.product_id
  account_name = var.account_name
  name         = "tf-example-dbms"
  allowed_ips  = var.allowed_ips

  lifecycle { prevent_destroy = true }
}
```

`allowed_ips` owns the **entire** allowlist, despite the API operation being named “add”. An update replaces the previous list;
external changes are drift and Terraform restores the configured set. Order does not cause differences. One resource owns all members;
do not manage the same list through a second resource or another state.

An empty PUT was rejected by the live API and retained its previous value. The provider therefore requires at least one canonical IPv4
host and rejects empty sets, CIDRs, IPv6 and null members before writes. Removing the final address is not supported. Destroy deletes
the DBMS service itself; it is not an allowlist-clear operation. A pre-existing observed empty list is readable, but a managed configuration
must supply a supported nonempty set. IP control-plane read-back is not a proof of packet filtering or successful database access.

## Schema

| Attribute | Type | Behavior |
| --- | --- | --- |
| `product_id` | required string | Exact nonempty creation ID. Changing it replaces the service. |
| `name` | required string | Alias, 4–32 Unicode code points. Replacement on change. |
| `account_name` | optional string | Initial 6–12 ASCII-letter account. Required to create, absent from Read. Adding/changing/removing this historical input replaces. |
| `description` | optional + computed string | Default empty, maximum 50 code points. Empty is omitted on create. Changes replace. |
| `allowed_ips` | required set(string) | Nonempty authoritative IPv4 host set. Changes update in place. |
| `id` | computed string | Exact positive decimal `service_idx`, without float64 rounding. |
| `status` | computed string | Observed control-plane state. `active` does not prove connectivity. |
| `product_type` | computed string | Observed `spec.type` tier, not an engine name. |
| `engine_version` | computed string | Observed `spec.ver`; not a configurable version or upgrade selector. |
| `address` | computed string | Default domain, without an inferred port or protocol. |
| `domains` | computed map(string) | All observed label-to-domain mappings. |

Local `timeouts` defaults: create/update/delete `5m`, read `1m`; all configured durations must be positive.
Changing only timeouts does not write to the service. Names/descriptions are literal; no HTML decoding or normalization occurs.

The catalog has duplicate product IDs across versions and even available rows with an empty creation ID (C30). The create API accepts
only `product_id`, not a version selector. Do not select an empty ID, deduplicate away version rows or assume a catalog version will be
provisioned. Inspect the observed version after creation. CPU/price/storage unit fields are excluded until verified.
DBMS catalog data sources are not registered yet.

## Replacement and import

Only the allowlist has a verified update endpoint. Changes to name, product, description or account history replace the entire service
and destroy its data. Ordinary replacements require a known different account name; for imported services without account history,
the user must explicitly choose a fresh name because Read cannot prove prior account identity. Use backups and a planned migration.
`create_before_destroy` can provision a new account first; it does not transfer data or clients for you.

Explicit `-replace`, taint and external deletion are separate Terraform Core behaviors and can bypass ordinary attribute comparisons.
Choose a fresh account for recreation. Account-name reuse rules after DBMS deletion are unverified; the hosting service's 24-hour rule
is **not** assumed to apply here. Plan-only tests inspect explicit replacement without executing same-name recreation.

Import by exact service ID after matching all readable settings:

```sh
terraform import iwinv_db_instance.example 123456789
terraform plan
```

The ID above is a synthetic placeholder. Omit `account_name` if its creation history is unavailable; it is never guessed from the domain.
Read restores product, name, description, allowed IPs, status, tier/version and domains. Initial account history and local timeouts are not
imported. Adding an account name later is a replacement request, not confirmation of an observed remote value. Never concurrently own
one service from two states. Use configuration matching the imported allowlist and description to obtain a no-change plan.

## Failure recovery

Create sends one request, saves a returned unambiguous ID before validating remaining receipt fields, and waits for active Read with
matching managed settings. The create receipt's domain is a string; Read's domain is an object. They use separate decoders.
Unverified receipts and delayed visibility preserve identity. A never-verified creation missing from the list produces a Read error;
its ID is retained, and deletion addresses the known identity once. A previously active service's confirmed absence can remove state.

Update errors, mismatched acknowledgements and convergence timeouts preserve prior state. Refresh before deciding another write;
the remote outcome may differ. API errors, HTTP 404, partial lists, duplicates and changed pagination metadata never establish absence.
Writes are not automatically replayed. Delete acknowledgement plus exact-ID absence is required; it does not independently prove billing termination.

See the [lifecycle decisions](../../design/en/dbms-lifecycle.md) and [verification evidence](../../design/en/contract-progress.md).
The separate webmail cleanup failure remains open and does not change DBMS's scoped evidence.

Sources: [create](https://iwinv-dbms.readme.io/reference/클라우드-dbms-생성),
[allowlist](https://iwinv-dbms.readme.io/reference/접근-허용-ip-추가),
[delete](https://iwinv-dbms.readme.io/reference/클라우드-dbms-삭제),
[products](https://iwinv-dbms.readme.io/reference/클라우드-dbms-상품-조회).
