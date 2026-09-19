---
page_title: "iwinv_content_cache Resource - iwinv"
subcategory: "Content delivery"
description: |-
  Manages a content-cache service and its authoritative referrer set.
---

# iwinv_content_cache (Resource)

[한국어](../ko/resources/content_cache.md) · [Development installation](../../design/en/development.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_content_cache-resource)

Development control-plane resource, verified with `cache_lite`; no Registry release yet.
This manages a service and its complete referrer list. FTP content, tenant API credentials, purge,
DNS records, HTTPS configuration and billing are outside this resource's verified behavior.
Other product variants need acceptance before support is claimed.

## Example

The [complete example](../../examples/resources/iwinv_content_cache/main.tf) requires a reviewed product,
a fresh account and an ephemeral password. Supply secrets through your private environment rather than HCL literals or checked-in files.

```hcl
variable "product_id" { type = string }
variable "account_name" { type = string }

variable "ftp_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "iwinv_content_cache" "example" {
  product_id          = var.product_id
  account_name        = var.account_name
  name                = "tf-example-cache"
  allowed_referrers   = ["media.example.com", "www.example.com"]
  ftp_password_wo     = var.ftp_password
  password_wo_version = 1

  lifecycle { prevent_destroy = true }
}
```

Terraform >=1.14.0 is the project baseline. `sensitive` alone does not prevent storage; use an ephemeral variable
with the write-only argument. The password is read from configuration and excluded from resource plan/state.
If it is placed literally in configuration, that source text may still be embedded in saved artifacts.

## Schema and ownership

| Attribute | Type | Behavior |
| --- | --- | --- |
| `product_id` | required string | Exact nonempty creation ID; changes replace. Null catalog IDs cannot create. |
| `account_name` | required string | Readable account, 6–12 ASCII letters/digits. Changes replace. |
| `name` | required string | Alias, 4–32 Unicode code points; changes replace. |
| `description` | optional + computed string | Default empty, maximum 50 code points. Empty omitted on create; changes replace. |
| `allowed_referrers` | optional + computed set(string) | Default empty. Complete authoritative set of lowercase ASCII DNS hostnames. Nonempty changes update in place; clearing requires replacement. |
| `ftp_password_wo` | optional sensitive write-only string | Required for creation/replacement; 7–20 non-space printable ASCII characters using at least two of letters/digits/symbols. Changing only this value does not trigger a write. |
| `password_wo_version` | optional int64 | Positive local version. Adding/changing/removing it replaces the entire service; it does not rotate a password in place. Omit unknown history during import. |
| `id` | computed string | Exact positive decimal `service_idx`; import ID. |
| `status` | computed string | Observed control-plane state. Active does not guarantee an unlocked service or FTP/content connectivity. |
| `ip_address` | computed string | Observed service IP. |
| `domain_name` | computed string | Observed domain string; no inferred port, protocol or DNS ownership. |
| `product_type` | computed string | Observed `spec.type`. It may say SINGLE for a SHARE catalog product; no isolation guarantee (C31). |

Local `timeouts` defaults: create/update/delete `5m`, read `1m`; durations must be positive. Timeout-only updates do not write remotely.
Names/descriptions remain literal, including Korean text and HTML-looking sequences.

The API operation called “add referrers” **replaces the complete list** (C22). One resource must own the whole set.
Do not split it into individual memberships or manage it from another state. External changes are drift; Terraform restores configuration.
Order is insignificant. Wildcards, IPs, URLs, paths, ports, IDNs, trailing dots and null members are rejected before writes.
Unsupported existing lists produce diagnostics rather than silently dropping members. Control-plane read-back does not verify HTTP filtering.

Use the read-only [product catalog](../data-sources/content_cache_products.md) to review nullable IDs and explicit creation choices.

## Empty sets, replacement and import

The create endpoint ignores the referrer argument, so creation first saves the parent ID and verifies the service, then applies a
nonempty list separately. Initially empty sets need no PUT. The API rejects empty PUTs: clearing a nonempty list therefore replaces
and deletes the **entire service and its content**, requiring a known different account and an initial password. Unknown whole sets
are conservatively treated as a possible clear when the prior set is nonempty. A known nonempty set with unknown members can update
once those members resolve. Inspect replacement plans; `prevent_destroy` in the example blocks them until explicitly removed.

Name/product/account/description/version changes also replace. Deletion is irreversible and the vendor prohibits account-name reuse
for **24 hours**. Choose a fresh account for every replacement. `create_before_destroy` creates the new account first but does not
migrate content or clients. Explicit `-replace`, taint and external deletion are separate Core paths and can bypass attribute comparisons;
plan-only tests inspect them without attempting prohibited same-name recreation. Change the account and supply credentials before applying.

Import after matching all readable settings:

```sh
terraform import iwinv_content_cache.example 123456789
terraform plan
```

This ID is a synthetic placeholder. Import restores the actual account, product, name, description, complete referrer set and computed fields.
Omit `ftp_password_wo` and `password_wo_version`; the password and local history cannot be recovered. Matching configuration then has a
no-change plan, and in-place referrer updates require no password. Do not retain version `1` from the creation example for an imported
service unless you intend replacement. One service must belong to one Terraform state.

## Failure recovery

Create sends one POST and records an exact returned ID before validating the remainder. Failed initialization or delayed visibility
retains the identity. Never-verified missing services yield a Read error; previously active services with confirmed exact-ID absence can
leave state. Partial/malformed lists, errors and ordinary HTTP 404 never mean deletion. A failed create may be tainted by Core: inspect
and reconcile the retained identity before accepting replacement.

Only the precisely classified cache PUT/DELETE “another operation in progress” rejection permits bounded retry. Every retry first
requires a successful exact-ID Read with the entire parent unchanged. Changed/missing parents, read failures and timeout stop retries.
Accepted requests, generic errors, uncertain transport failures and create requests are never automatically replayed. Prior state is
retained on failed updates or deletion waits; refresh before deciding how to recover. An acknowledged deletion plus exact-ID absence
verifies control-plane cleanup, not independent termination of billing.

See [lifecycle decisions](../../design/en/cache-lifecycle.md) and [verification evidence](../../design/en/contract-progress.md).
The separate unresolved webmail cleanup is not covered by cache acceptance.

Sources: [create](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-생성),
[referrers](https://iwinv-cache.readme.io/reference/레퍼러-추가),
[delete](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-삭제).
