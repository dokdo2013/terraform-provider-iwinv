---
page_title: "iwinv_ssh_keys Data Source - iwinv"
subcategory: "Compute"
description: |-
  List existing SSH key references without exposing key material or modifying keys.
---

# iwinv_ssh_keys

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/ssh_keys.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_ssh_keys-data-source)

List existing SSH key references without exposing key material or modifying keys.

Development provider; no Registry release yet. Start with [provider setup](../index.md).

## Example Usage

```hcl
data "iwinv_ssh_keys" "available" {}

output "available_ssh_keys" {
  value = data.iwinv_ssh_keys.available.keys
}
```

## Argument Reference

No service-specific arguments. There is no name filter.

## Attribute Reference

| Attribute | Type | Meaning |
| --- | --- | --- |
| `ids` | `list(string)` | Exact ssh_key_id values in lexical order. |
| `keys` | `list(object)` | String id and name for each key, in the same order as ids. |

All attributes are computed; do not assign values to them in configuration.

## Behavior and limitations

The provider validates every ten-row page until a short page, including an extra empty page when needed. Missing/null names, empty IDs, duplicate IDs, changed or invalid pagination metadata and later-page errors fail the complete read. Empty names and duplicate display names are preserved; they are not identifiers. The maximum is 1,000 pages. A valid empty array gives empty ids/keys. Public/private keys, fingerprints, downloads and other arbitrary response fields are excluded. Use [ssh_key](ssh_key.md) for exact-ID selection. No key generation/upload/deletion or server-side key installation is performed or verified.

Read-only; import does not apply. API errors do not become empty results.

[Example](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/examples/data-sources/iwinv_ssh_keys/main.tf) · [Evidence](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/contract-progress.md)

[Official API](https://iwinv-common.readme.io/reference/get_new-endpoint-1-1)
