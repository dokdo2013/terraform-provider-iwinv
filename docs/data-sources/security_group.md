---
page_title: "iwinv_security_group Data Source - iwinv"
subcategory: "Networking"
description: |-
  Read existing iwinv security group attributes.
---

# iwinv_security_group

[한국어](../ko/data-sources/security_group.md)

Development provider; no Registry release yet. Reads one exact existing group ID without name matching or automatic selection.

```hcl
variable "security_group_id" {
  type        = string
  description = "Exact existing iwinv FIREWALL ID."
}

data "iwinv_security_group" "selected" {
  id = var.security_group_id
}
```

`id` is a required string containing the exact `FIREWALL-...` ID. Zero, multiple or mismatched results are errors.

Read attributes:

| Attribute | Type | Meaning |
| --- | --- | --- |
| `name` | string | Literal API title; names need not be unique. |
| `description` | nullable string | HTML-unescaped exactly once; null and empty remain distinct. |
| `allow_icmp` | bool | Maps the API Y/N ICMP flag. |

Rules, attached instances and arbitrary response fields are excluded. Reads neither own nor mutate remote objects and do not support import. Use the [security group resource](../resources/security_group.md) to manage attributes.

The list validates counts, page numbers/sizes, fields and unique IDs across all pages before returning. Authentication/HTTP errors and later-page failures never become empty or partial results. Sorting stabilizes response ordering but cannot guarantee an atomic snapshot during concurrent inventory changes. Unknown ID references are deferred by Terraform until apply.

T073: synthetic Core tests cover 51-row pagination, reordered responses, duplicate names, null/empty descriptions, exact IDs, invalid/missing IDs, permission/late-page errors and unknown references. Live acceptance uses one run-owned fixture to compare list/detail, round-trip descriptions, refresh after an external rename and verify no-change plans, then deletes that fixture. More than 50 live groups, attachments and firewall traffic enforcement remain separate checks.

[Example](../../examples/data-sources/iwinv_security_group/main.tf) · [Verification](../../design/en/contract-progress.md)

Sources: [official list](https://iwinv.readme.io/reference/get_v1-security-groups), [official detail](https://iwinv.readme.io/reference/get_v1-security-groups-id).
