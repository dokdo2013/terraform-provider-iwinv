---
page_title: "iwinv_webmail_products Data Source - iwinv"
subcategory: "Webmail"
description: |-
  Read the full webmail product catalog, including coming-soon rows with empty IDs.
---

# iwinv_webmail_products

[한국어](../ko/data-sources/webmail_products.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_webmail_products-data-source)

Reads catalog metadata only. Development provider; no Registry release yet.

```hcl
data "iwinv_webmail_products" "all" {}

output "webmail_products" {
  value = data.iwinv_webmail_products.all.products
}
```

There are no configurable attributes or documented filters. `products` is a computed list of objects:

| Attribute | Type | Meaning |
| --- | --- | --- |
| `product_id` | string | Exact catalog ID. Empty strings are preserved and are not selectable creation IDs. |
| `name` | string | Literal product name. Names need not uniquely identify nonempty IDs. |
| `status` | string | Observed catalog availability, not service provisioning status. |
| `product_type` | string | Literal `spec.type`; it is not a service-isolation guarantee. |

Rows sort lexically by ID, product type and name. Empty IDs sort first: never use the first row as a default product.
The observed catalog contained twelve SHARE rows: four available products with distinct IDs and eight coming-soon rows with empty IDs.
All rows remain visible; the provider does not invent placeholder IDs, collapse products by empty ID or treat availability as successful provisioning.

The endpoint returned HTTP 200 without count or pagination metadata. A successful empty array is valid. Missing/null/non-string IDs,
missing/empty name, status or product type, duplicate nonempty IDs, ambiguous empty-ID rows with the same type/name, changed pagination,
inconsistent optional count and API errors fail the entire read. It never publishes a partial catalog on failure. Price, VAT, disk and traffic
fields are excluded pending independent unit/billing contracts. No remote ownership, import, credentials or mailbox contents are exposed.

**Webmail service and mailbox resources remain unsupported.** Prior lifecycle testing found service visibility/deletion inconsistencies
and no independently verified mailbox Read. A separate test service still has unresolved cleanup T056. A valid product ID does not resolve
those blockers. This catalog data source sends only GET requests; it neither creates a service/account nor sends mail or changes DNS.

T075 covers strict adapter and Terraform Core checks, empty IDs, distinct placeholders, duplicate display names, literal Korean/HTML-like names,
response reordering, malformed/empty/error responses and a live complete read followed by a no-change plan. Catalog growth can change output.

[Example](../../examples/data-sources/iwinv_webmail_products/main.tf) · [Evidence](../../design/en/contract-progress.md)

Source: [official product endpoint](https://iwinv-webmail.readme.io/reference/웹-메일-상품-조회).
