---
page_title: "iwinv_instance_types Data Source - iwinv"
subcategory: "Compute"
description: |-
  Read the exact compute product IDs exposed by the flavors API.
---

# iwinv_instance_types

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/instance_types.md)

Read the exact compute product IDs exposed by the flavors API.

Development provider; no Registry release yet. Start with [provider setup](../index.md).

## Example Usage

```hcl
data "iwinv_instance_types" "available" {}

output "instance_type_ids" {
  value = data.iwinv_instance_types.available.ids
}
```

## Argument Reference

No service-specific arguments. No zone, price or capacity filter is implemented.

## Attribute Reference

| Attribute | Type | Meaning |
| --- | --- | --- |
| `ids` | `list(string)` | Exact flavor_id values from every page, sorted lexically. |

All attributes are computed; do not assign values to them in configuration.

## Behavior and limitations

Dots and other supported ID characters are preserved; IDs are not converted into AWS instance types. Ten-row pages are validated against count, page number, page size and a consistent total. Collection ends only when the total is satisfied; duplicate IDs, changed totals, early short pages or later failures reject the entire result. The limit is 1,000 pages, and no snapshot token guarantees atomicity during concurrent changes. A valid zero-total empty catalog is empty. Use [instance_type](instance_type.md) for one product display name. Do not choose by list position or interpret catalog membership as quota, stock, pricing or zone compatibility.

Read-only; import does not apply. API errors do not become empty results.

[Example](../../examples/data-sources/iwinv_catalogs/main.tf) · [Evidence](../../design/en/contract-progress.md)

[Official API](https://iwinv.readme.io/reference/getv1flavors)
