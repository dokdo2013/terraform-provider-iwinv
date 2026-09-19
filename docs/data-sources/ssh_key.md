---
page_title: "iwinv_ssh_key Data Source - iwinv"
subcategory: "Compute"
description: |-
  Find one existing SSH key by its exact ID after validating the complete key list.
---

# iwinv_ssh_key

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/ssh_key.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_ssh_key-data-source)

Find one existing SSH key by its exact ID after validating the complete key list.

Development provider; no Registry release yet. Start with [provider setup](../index.md).

## Example Usage

```hcl
variable "ssh_key_id" {
  type = string
}

data "iwinv_ssh_key" "selected" {
  id = var.ssh_key_id
}

output "ssh_key_name" {
  value = data.iwinv_ssh_key.selected.name
}
```

## Argument Reference

`id` (required string): exact ssh_key_id from [ssh_keys](ssh_keys.md). It must be known and non-empty when Read executes. The ID is compared as an opaque string, not inserted into a guessed detail endpoint.

## Attribute Reference

| Attribute | Type | Meaning |
| --- | --- | --- |
| `id` | `string` | The configured exact SSH key reference ID. |
| `name` | `string` | API display name; empty names are preserved. |

`id` is required configuration; the other attributes are computed.

## Behavior and limitations

Only a list API is established, so even an early match is not returned until every page has been validated. The same pagination, duplicate-ID and failure rules as [ssh_keys](ssh_keys.md) apply. A missing ID is an error; duplicate display names are allowed but never used to choose a key. No key material is returned, and this data source cannot recover a lost private key. It neither imports nor owns a key. Live read/no-change-plan acceptance covers references only, not key mutation or successful SSH login to a server.

Read-only; import does not apply. API errors do not become empty results.

[Example](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/examples/data-sources/iwinv_ssh_keys/main.tf) · [Evidence](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/contract-progress.md)

[Official API](https://iwinv-common.readme.io/reference/get_new-endpoint-1-1)
