---
page_title: "iwinv_content_cache_products Data Source - iwinv"
subcategory: "Content delivery"
description: |-
  Reads complete cache product rows while preserving null and empty IDs.
---

# iwinv_content_cache_products (Data Source)

[한국어](../ko/data-sources/content_cache_products.md) · [Development installation](../../design/en/development.md)

Reads the content-cache product catalog without creating a service. Development provider only; no Registry release exists yet.

```hcl
data "iwinv_content_cache_products" "all" {}
data "iwinv_content_cache_products" "shared" {
  product_type = "SHARE"
}
output "cache_products" {
  value = data.iwinv_content_cache_products.shared.products
}
```

| Attribute | Type | Meaning |
| --- | --- | --- |
| `product_type` | optional string | Exact `SHARE` or `SINGLE` filter; omit for all visible types. |
| `products` | computed list(object) | Complete rows, sorted with null IDs first, then lexical ID, type and name. |

Each row has nullable `product_id` and observed string `name`, `status`, `product_type`. Null IDs are real coming-soon catalog entries;
empty IDs are also preserved distinctly. Neither can create a service. Review a nonempty ID explicitly before using it with
[`iwinv_content_cache`](../resources/content_cache.md). No first-item selection, fabricated ID or flattened `ids` set is provided.
Ordering is not a recommendation, and visible/available products do not establish provisioning eligibility.

Omitted/null filters are unfiltered. Empty or differently cased strings are errors. Unknown filters must resolve before Read;
unknown never silently becomes an unfiltered request. Returned rows must match a requested type. C31 records that catalog `SHARE`
can become observed service `SINGLE`; the provider preserves both observations rather than inferring an isolation guarantee.
Price, disk, traffic and other unit-dependent fields remain excluded pending verification.

Empty arrays are valid. Missing IDs, non-string/non-null IDs, malformed rows, duplicate nonempty IDs, duplicate unselectable
name/type combinations, filter mismatches, changed pagination/count metadata and API errors fail the whole Read.
No import or remote ownership applies. T067 covers these synthetic cases and live unfiltered/SHARE/SINGLE reads followed by
no-change plans, including nullable IDs. This is catalog acceptance, not acceptance of every product or content/FTP connectivity.

[Read-only example](../../examples/data-sources/iwinv_content_cache_products/main.tf) ·
[Verification](../../design/en/contract-progress.md)

Source: [official cache product API](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-상품-조회).
