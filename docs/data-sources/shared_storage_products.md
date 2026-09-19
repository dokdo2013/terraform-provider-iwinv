---
page_title: "iwinv_shared_storage_products Data Source - iwinv"
subcategory: "Storage"
description: |-
  Reads the API NAS product catalog with nullable versions and capacity bounds.
---

# iwinv_shared_storage_products (Data Source)

[한국어](../ko/data-sources/shared_storage_products.md) · [Development installation](../../design/en/development.md)

Reads all API NAS products without creating storage. Development provider; no Registry release yet.
No filter arguments are documented for this endpoint, so this data source has no configurable attributes.

```hcl
data "iwinv_shared_storage_products" "all" {}

output "storage_products" {
  value = data.iwinv_shared_storage_products.all.products
}
```

`products` is a computed list of objects, sorted lexically by product ID then name. Empty IDs sort first.
Each row contains:

| Attribute | Type | Meaning |
| --- | --- | --- |
| `product_id` | string | Exact creation ID; empty IDs are preserved and cannot create a service. |
| `name` | string | Literal observed product name. |
| `status` | string | Observed availability status. |
| `version` | nullable string | Observed version; null and empty string remain distinct. Not an independent creation selector. |
| `minimum_size_gb` | int64 | Minimum capacity in documented GB; coming-soon rows may contain zero. |
| `maximum_size_gb` | int64 | Maximum capacity in documented GB; not a resize capability. |

Review a nonempty ID and its capacity bounds before using [`iwinv_shared_storage`](../resources/shared_storage.md).
Do not select the first row as a default product. Availability and valid bounds do not independently establish provisioning eligibility.
The resource currently replaces storage when capacity changes; the catalog does not imply in-place expansion or migration.
Pricing and arbitrary response fields are excluded. No import or remote ownership applies.

Empty arrays are valid. Missing/null/non-string IDs, missing/non-string versions except explicit null, malformed or missing integer
bounds, negative/reversed bounds, duplicate nonempty IDs or duplicate empty-ID names, pagination/count metadata changes and API errors
fail the whole Read. Coming-soon rows with distinct names remain separate. List order changes do not produce state changes.

T070 covers synthetic malformed/empty/order cases and live catalog reads followed by a no-change plan. Live observations include
an available api_nas with 100–2000 GB bounds and coming-soon rows with empty IDs, null versions and zero bounds. Other products,
file access, pricing and billing remain separate verification requirements.

[Read-only example](../../examples/data-sources/iwinv_shared_storage_products/main.tf) ·
[Verification](../../design/en/contract-progress.md)

Source: [official API NAS products](https://iwinv-api-nas.readme.io/reference/공유-스토리지-상품-조회).
