---
page_title: "iwinv_webhosting_products Data Source - iwinv"
subcategory: "Hosting"
description: |-
  Reads the API-visible hosting product catalog with an optional SHARE or SINGLE filter.
---

# iwinv_webhosting_products (Data Source)

[한국어](../ko/data-sources/webhosting_products.md) · [Development installation](../../design/en/development.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_webhosting_products-data-source)

Lists hosting products without creating or adopting a service. Development provider only; no Registry release yet.
Catalog membership and status do not guarantee successful creation. Review a product and its server choices explicitly.

```hcl
data "iwinv_webhosting_products" "shared" {
  type = "SHARE"
}
output "hosting_products" {
  value = data.iwinv_webhosting_products.shared.products
}
```

Omit `type` for the unfiltered catalog. Only exact uppercase `SHARE` or `SINGLE` values are supported;
an empty string is not the same as omission. The API filter is sent in the query, not inferred from product names.

| Attribute | Type | Meaning |
| --- | --- | --- |
| `type` | optional string | Exact API filter; omitted/null means all visible products. |
| `ids` | computed list(string) | Product IDs sorted lexically. |
| `products` | computed list(object) | Product metadata in the same order as `ids`. |

Each product contains `id`, `name`, `status`, `type`, `php_versions` (sorted exact labels), `allow_custom_domain`,
`enable_domain_folder`, `max_domain_count` and `domain_edit_interval_days`. Counts and day intervals are integers.
Display text is preserved literally, including HTML entities. Prices, VAT, disk and traffic are excluded because their units remain unverified.
Do not interpret list position as a recommendation or choose index zero for production provisioning.

Empty lists are valid. Duplicate IDs, malformed rows, invalid fields or changed pagination/count metadata fail the whole read;
partial results are never returned as complete. Errors and HTTP 404 are not empty catalogs.
This read-only data source has no import or remote ownership.

Use [server choices](webhosting_servers.md) with an explicitly selected product ID, then configure
[the hosting resource](../resources/webhosting.md) with a selected server ID. The [combined example](../../examples/data-sources/iwinv_webhosting_catalogs/main.tf)
only reads catalogs. T061 covers synthetic invalid/empty/error contracts and live reads followed by a no-change plan;
actual service lifecycle coverage remains limited to the resource's documented product/version scope.

Source: [official product catalog](https://iwinv-hosting.readme.io/reference/공유형단독형).
