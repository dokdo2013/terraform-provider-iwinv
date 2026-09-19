---
page_title: "iwinv_security_group Data Source - iwinv"
subcategory: "Networking"
description: |-
  Read existing iwinv security group attributes.
---

# iwinv_security_group

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/security_group.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_security_group-data-source)

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

This lookup calls the exact-ID detail endpoint once; it does not traverse the group list. The response must be HTTP 200 with a matching count, exactly one matching ID and valid attribute fields, without pagination metadata. An empty successful response produces a not-found diagnostic; authentication/HTTP errors are also errors. Unknown ID references are deferred by Terraform until apply. Use [security_groups](security_groups.md) for complete paginated inventory.

T073: synthetic Core tests cover 51-row pagination, reordered responses, duplicate names, null/empty descriptions, exact IDs, invalid/missing IDs, permission/late-page errors and unknown references. Live acceptance uses one run-owned fixture to compare list/detail, round-trip descriptions, refresh after an external rename and verify no-change plans, then deletes that fixture. More than 50 live groups, attachments and firewall traffic enforcement remain separate checks.

[Example](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/examples/data-sources/iwinv_security_group/main.tf) · [Verification](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/contract-progress.md)

Sources: [official list](https://iwinv.readme.io/reference/get_v1-security-groups), [official detail](https://iwinv.readme.io/reference/get_v1-security-groups-id).
