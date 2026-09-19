---
page_title: "iwinv_db_instance_products Data Source - iwinv"
subcategory: "Database"
description: |-
  Reads DBMS catalog rows while preserving empty and repeated product IDs.
---

# iwinv_db_instance_products (Data Source)

[한국어](../ko/data-sources/db_instance_products.md) · [Development installation](../../design/en/development.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_db_instance_products-data-source)

Reads the DBMS product catalog without creating a database. Development provider only; no Registry release exists yet.

```hcl
data "iwinv_db_instance_products" "all" {}
data "iwinv_db_instance_products" "redis" {
  product_type = "STD"
  engine       = "redis"
}
output "db_products" {
  value = data.iwinv_db_instance_products.redis.products
}
```

| Attribute | Type | Meaning |
| --- | --- | --- |
| `product_type` | optional string | Exact `STD` or `HM` query filter. Omit for all visible tiers, including other tier labels. |
| `engine` | optional string | Exact `MySQL`, `MariaDB`, `mongoDB`, `MS-SQL`, `redis` or `PostgreSQL` API query filter. |
| `products` | computed list(object) | Complete rows sorted lexically by product ID, tier and version. |

Omitted/null filters are unfiltered; empty strings are errors. Unknown filters must resolve before Read and never fall back to an unfiltered request.
Each row has `product_id`, `name`, `status`, `product_type` and `engine_version`. These are literal observed strings.
The response does not separately echo an engine identifier; `engine` is a query input, and `product_type` is a tier.
Price/VAT and CPU/memory/disk quantities are excluded until their units and meaning are verified.

**Review creation IDs explicitly.** C30 records available rows with empty IDs and repeated IDs across versions. This data source retains
both rather than silently dropping rows, deduplicating by ID or inventing identifiers. No flattened `ids` set or automatic first-item selection is provided.
Sorting is not an availability or version recommendation. Check the whole unfiltered catalog when reviewing whether an ID is ambiguous;
a filtered row alone may hide another version with the same ID. Empty IDs cannot create a resource.

The [DBMS resource](../resources/db_instance.md) accepts `product_id`, but the public POST API has no version selector.
Filtering on an engine does not supply a creation-time engine/version constraint. Catalog status does not establish provisioning eligibility.
Do not interpret a row as a promise that a specific engine version will be created.

Empty arrays are valid. Missing/null IDs, malformed rows, exact duplicate ID/tier/version tuples, mismatched type filters and changed
pagination/count metadata fail the entire Read. HTTP/API failures are errors, not empty catalogs. No import or remote ownership applies.

T064 covers synthetic errors/empty results/unknown and invalid filters, preservation of empty and repeated IDs, and live reads followed by
no-change plans for the unfiltered catalog, all six engine filters, both tier filters and STD/redis together. These tests do not provision
all engine variants or independently verify engine identity/connectivity. See the [read-only example](../../examples/data-sources/iwinv_db_instance_products/main.tf).

Source: [official DBMS product API](https://iwinv-dbms.readme.io/reference/클라우드-dbms-상품-조회).
