---
page_title: "iwinv_availability_zones Data Source - iwinv"
subcategory: "Compute"
description: |-
  Read the API-visible zone catalog without managing zones or creating servers.
---

# iwinv_availability_zones

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/availability_zones.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_availability_zones-data-source)

Read the API-visible zone catalog without managing zones or creating servers.

Development provider; no Registry release yet. Start with [provider setup](../index.md).

## Example Usage

```hcl
data "iwinv_availability_zones" "available" {}

output "zone_ids" {
  value = data.iwinv_availability_zones.available.zone_ids
}
```

## Argument Reference

No service-specific arguments. There is no region or status filter.

## Attribute Reference

| Attribute | Type | Meaning |
| --- | --- | --- |
| `zone_ids` | `list(string)` | Exact API IDs, sorted lexically. |
| `names` | `list(string)` | Display names aligned with zone_ids. |
| `zones` | `list(object)` | Rows with string id, name and status, aligned with zone_ids. |

All attributes are computed; do not assign values to them in configuration.

## Behavior and limitations

The provider preserves IDs and status strings exactly. It checks every row, the response count and ID uniqueness. A valid empty array is an empty catalog; malformed data or API errors fail the read. Names and status are not translated into AWS regions or availability guarantees. Zone visibility does not establish that every console server is accessible through this API. Compute provisioning remains unimplemented.

Read-only; import does not apply. API errors do not become empty results.

[Example](../../examples/data-sources/iwinv_availability_zones/main.tf) · [Evidence](../../design/en/contract-progress.md)

[Official API](https://iwinv.readme.io/reference/getv1zones)
