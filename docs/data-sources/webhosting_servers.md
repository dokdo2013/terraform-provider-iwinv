---
page_title: "iwinv_webhosting_servers Data Source - iwinv"
subcategory: "Hosting"
description: |-
  Reads exact server choices for one hosting product.
---

# iwinv_webhosting_servers (Data Source)

[한국어](../ko/data-sources/webhosting_servers.md) · [Development installation](../../design/en/development.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_webhosting_servers-data-source)

Reads server choices for one explicitly selected hosting product; does not provision a service. Development provider only.

```hcl
variable "hosting_product_id" { type = string }
data "iwinv_webhosting_servers" "selected" {
  product_id = var.hosting_product_id
}
output "hosting_server_choices" {
  value = data.iwinv_webhosting_servers.selected.servers
}
```

| Attribute | Type | Meaning |
| --- | --- | --- |
| `product_id` | required string | Nonempty exact ID from the [product catalog](webhosting_products.md); sent as the required query. |
| `ids` | computed list(string) | Exact positive decimal server `idx` values, sorted lexically. |
| `servers` | computed list(object) | `id`, `charset`, `php_version`, `database` and `program` strings, in the same order as `ids`. |

Server IDs remain exact strings, even above 2^53. Lexical order is not numeric or preference order.
A server ID is a creation selector, distinct from a provisioned service's `service_idx`; hosting service Read cannot reconstruct it.
`database` and `program` may be empty strings. PHP labels and other values are preserved as returned, without version inference.

Choose a server after reviewing charset, PHP and database requirements. Catalog membership does not guarantee availability,
compatibility with arbitrary inputs, or successful provisioning. Do not silently pick the first row.
The [combined example](../../examples/data-sources/iwinv_webhosting_catalogs/main.tf) exposes the choices for review.

The full array is validated before publishing results. An empty list is valid; malformed/duplicate rows, changed pagination/count
metadata and API errors fail the read. No partial list or HTTP 404 is treated as successful absence.
This data source has no import, mutation or ownership. T061 records live reads/no-change plans and synthetic contract checks.

Source: [official server choices](https://iwinv-hosting.readme.io/reference/상품-상세-조회).
