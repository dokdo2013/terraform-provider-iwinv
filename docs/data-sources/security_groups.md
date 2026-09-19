---
page_title: "iwinv_security_groups Data Source - iwinv"
subcategory: "Networking"
description: |-
  Read existing iwinv security group attributes.
---

# iwinv_security_groups

[한국어](../ko/data-sources/security_groups.md)

Development provider; no Registry release yet. Reads all API-visible groups. There are no input arguments or name filters.

```hcl
data "iwinv_security_groups" "all" {}
```

`ids` is a computed `list(string)` and `groups` is a computed list of objects. Both use lexical ID order; corresponding positions identify the same group. Empty inventories return empty lists.

Read attributes (each `groups` row):

| Attribute | Type | Meaning |
| --- | --- | --- |
| `id` | string | Exact remote identity. |
| `name` | string | Literal API title; names need not be unique. |
| `description` | nullable string | HTML-unescaped exactly once; null and empty remain distinct. |
| `allow_icmp` | bool | Maps the API Y/N ICMP flag. |

Rules, attached instances and arbitrary response fields are excluded. Reads neither own nor mutate remote objects and do not support import. Use the [security group resource](../resources/security_group.md) to manage attributes.

The list validates counts, page numbers/sizes, fields and unique IDs across all pages before returning. Authentication/HTTP errors and later-page failures never become empty or partial results. Sorting stabilizes response ordering but cannot guarantee an atomic snapshot during concurrent inventory changes. Unknown ID references are deferred by Terraform until apply.

T073: synthetic Core tests cover 51-row pagination, reordered responses, duplicate names, null/empty descriptions, exact IDs, invalid/missing IDs, permission/late-page errors and unknown references. Live acceptance uses one run-owned fixture to compare list/detail, round-trip descriptions, refresh after an external rename and verify no-change plans, then deletes that fixture. More than 50 live groups, attachments and firewall traffic enforcement remain separate checks.

[Example](../../examples/data-sources/iwinv_security_groups/main.tf) · [Verification](../../design/en/contract-progress.md)

Sources: [official list](https://iwinv.readme.io/reference/get_v1-security-groups), [official detail](https://iwinv.readme.io/reference/get_v1-security-groups-id).
