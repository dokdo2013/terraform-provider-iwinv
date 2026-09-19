---
page_title: "iwinv_security_group_ingress_rule Resource - iwinv"
subcategory: "Networking"
description: |-
  Manages one ingress TCP/UDP rule in an iwinv security group.
---

# iwinv_security_group_ingress_rule (Resource)

[한국어](../ko/resources/security_group_ingress_rule.md) · [Installation](../../design/en/development.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_security_group_ingress_rule-resource)

Development resource for one ingress rule. It owns the rule's attributes, not its parent group or other rules.
There is no Registry release yet. Live control-plane lifecycle tests passed on dedicated, unattached groups; actual packet filtering remains unverified.

## Example

```hcl
resource "iwinv_security_group" "example" {
  name = "tf-example-group"
}

resource "iwinv_security_group_ingress_rule" "example" {
  security_group_id = iwinv_security_group.example.id
  name              = "example-ingress"
  description       = "Example rule"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = "192.0.2.0/24"
}
```

[Complete runnable development example](../../examples/resources/iwinv_security_group_ingress_rule/main.tf).
The parent reference gives Terraform a dependency: delete managed rules before deleting their group.

## Schema

| Attribute | Type | Behavior |
| --- | --- | --- |
| `security_group_id` | string, required | Exact parent `firewall_id`; changing it replaces the rule. |
| `name` | string, required | 1–25 Unicode code points; updated in place. |
| `description` | string, optional + computed | Default empty, at most 25 code points; clearing a nonempty value replaces the rule. |
| `ip_protocol` | string, required | `tcp` or `udp`; updated in place. |
| `from_port`, `to_port` | int64, required | 1–65535, inclusive; `to_port >= from_port`. Equal values select one port. Updated in place. |
| `cidr_ipv4` | string, required | IPv4 CIDR, preserving host bits exactly; updated in place. |
| `id` | string, computed | Composite `security_group_id/rule_id` identity. |
| `rule_id` | string, computed | Exact positive decimal API rule ID, without floating-point conversion. |
| `direction` | string, computed | Observed direction; the resource type fixes its desired value. |

Optional `timeouts` block: `create`, `update`, `delete` default to `5m`; `read` defaults to `1m`. Use positive duration strings.
No tags, IPv6, ICMP rule protocol or source-group references are exposed. ICMP is a separate parent-group flag.
The API rejects port 0. Names and descriptions retain literal HTML entities; empty API descriptions returned as null become empty Terraform strings.
API multibyte length boundaries and packet semantics have not been exhaustively verified.

## Import and lifecycle

```sh
terraform import iwinv_security_group_ingress_rule.example FIREWALL-REPLACE_WITH_YOUR_ID/123
terraform plan
```

Both ID components above are placeholders. Use the exact parent and rule IDs, and match configuration to the imported attributes.
Import restores all supported attributes, including empty descriptions. Local timeout settings are not imported.
If imported into the wrong direction resource, plan shows the direction change; choose the correct resource type before applying.

The resource restores externally changed direction to `ingress` with an in-place update and displays that change in plan.
Setting a nonempty description is an in-place update. Clearing it, removing a previously nonempty argument, or changing the parent requires replacement.
A new description that is unknown during planning also conservatively requires replacement when the prior description is nonempty.
This prevents discovering a new replacement requirement only during apply; use known description values to retain in-place updates.

Read the shared [rule lifecycle and recovery guide](../guides/security_group_rules.md) before using replacements, import or cleanup.
Verification: T058; [live and synthetic evidence](../../design/en/contract-progress.md).
