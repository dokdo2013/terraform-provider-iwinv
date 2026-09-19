---
page_title: "iwinv_instance_type Data Source - iwinv"
subcategory: "Compute"
description: |-
  Read the display name for one exact compute product (API flavor_id).
---

# iwinv_instance_type

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/instance_type.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_instance_type-data-source)

Read the display name for one exact compute product (API flavor_id).

Development provider; no Registry release yet. Start with [provider setup](../index.md).

## Example Usage

```hcl
variable "instance_type_id" {
  type = string
}

data "iwinv_instance_type" "selected" {
  id = var.instance_type_id
}

output "instance_type_name" {
  value = data.iwinv_instance_type.selected.name
}
```

## Argument Reference

`id` (required string): exact flavor_id from [instance_types](instance_types.md). It must be known and non-empty when Read executes and match `[A-Za-z0-9_-][A-Za-z0-9_.-]*`. Dots are preserved. Unsupported path characters are rejected before a request.

## Attribute Reference

| Attribute | Type | Meaning |
| --- | --- | --- |
| `id` | `string` | The configured exact ID, verified against the response. |
| `name` | `string` | API product display name. |

`id` is required configuration; the other attributes are computed.

## Behavior and limitations

Exactly one matching row with a non-empty name is required. An empty successful API response is a failed detail lookup, not a null product or permission to create one. The provider does not choose the first match or expose unverified CPU, memory, price or capacity fields. Live lookup/no-change-plan evidence covers the current limited contract, not instance creation, resize or per-zone availability. This is a read-only reference with no remote ownership or import.

Read-only; import does not apply. API errors do not become empty results.

[Example](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/examples/data-sources/iwinv_catalogs/main.tf) · [Evidence](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/contract-progress.md)

[Official API](https://iwinv.readme.io/reference/getv1flavorsflavorid)
