---
page_title: "iwinv_security_group Resource - iwinv"
subcategory: "Networking"
description: |-
  Manages the name, nonempty description and ICMP flag of an iwinv security group.
---

# iwinv_security_group (Resource)

[한국어](../ko/resources/security_group.md) · [Development installation](../../design/en/development.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_security_group-resource)

Development support for security-group attributes. No Registry release exists yet.
This resource owns `name`, `description` and `allow_icmp` for one exact `firewall_id`.
It does not manage inline rules or instance attachments. Use the independent [rule resources](../guides/security_group_rules.md); attachment resources are not yet implemented.
Live lifecycle evidence covers dedicated, unattached groups. Independent managed rules are deleted before their parents in integration tests. Packet filtering, attached-group deletion and physical rule cascades remain unverified.
Use a dedicated group within this scope. Review dependencies before destroy; do not assume excluded child objects survive deletion of their parent.

## Example

```hcl
resource "iwinv_security_group" "example" {
  name        = "tf-example-group"
  description = "Managed by Terraform"
  allow_icmp  = false

  timeouts {
    create = "5m"
    read   = "1m"
    update = "5m"
    delete = "5m"
  }
}
```

The [complete example](../../examples/resources/iwinv_security_group/main.tf) requires development installation and environment credentials.
`apply` creates a real cloud object; `destroy` removes it. Keep state outside public Git and review plans before applying.

## Schema

| Attribute | Type | Behavior |
| --- | --- | --- |
| `name` | string, required | 4–32 Unicode code points, using documented limits. Updated in place. |
| `description` | string, optional + computed | 1–50 Unicode code points. Defaults to `Managed by Terraform`. Updated in place. |
| `allow_icmp` | bool, optional + computed | Defaults to `false`; writes explicit API `Y`/`N`. |
| `id` | string, computed | Exact API `firewall_id`; stable across supported updates. |

API length boundaries have not been exhaustively tested with multibyte text. No trimming, Unicode normalization or URL encoding is applied.
The observed API HTML-escapes descriptions; the provider decodes exactly once and leaves names verbatim.
Literal entities such as `&amp;` remain literal entities in configuration and state.
Explicit empty descriptions are rejected before apply. Removing the argument resets it to the default; it does not clear the remote value.
API requests omit `content` when a name/ICMP update leaves the description unchanged.

Optional `timeouts` block: `create`, `update`, `delete` default to `5m`; `read` defaults to `1m`.
Values must be positive duration strings. Local timeout changes alone do not issue update requests.
A timeout bounds the whole operation, including polls and client rate limiting; it does not undo a remote write.

## Import

Match the existing supported values in configuration, then import the exact `firewall_id`:

```sh
terraform import iwinv_security_group.example FIREWALL-REPLACE_WITH_YOUR_ID
terraform plan
```

The ID above is a placeholder. Names are never import selectors. IDs must be a single `FIREWALL-` segment with letters, digits, `_` or `-` in the suffix.
Import reads the three attributes. It does not create a group or assume ownership of rules/attachments.
A matching configuration produces an empty plan. Local timeout settings have no remote representation and are not imported.
Legacy groups with null/empty descriptions can be read, but configuring an explicit empty description is unsupported; the default or a nonempty value will plan an update.
Do not manage the same group from two states.

## Failures and recovery

Writes are never automatically retried. Create stores an unambiguous returned ID before validating the rest of the receipt or waiting for read-back.
If creation then fails, Terraform can mark that ID tainted and propose replacement. Inspect the actual object and plan before another apply.
After confirming that the existing object matches the intended configuration, use Terraform's documented recovery procedures to retain it or explicitly destroy it.
If no ID was returned, reconcile the private request record and account inventory first; the provider never adopts a same-name object or blindly repeats the request.

Create/update finish when exact-ID detail reads match the planned attributes. Only successful incomplete reads are polled.
API errors, HTTP 404, malformed results and unexpected IDs remain errors. They do not remove state.
Only this endpoint's verified HTTP 200 empty array with count zero establishes absence.
Delete reads before writing, sends one DELETE, and then waits for that absence; acknowledgement alone is insufficient.
Failed updates/deletes retain the prior state so a later refresh can reconcile the outcome.
API absence is not independent billing confirmation.

## Verification and limits

Synthetic Terraform CLI tests cover lifecycle, import, drift, timeout-only edits and failed-create cleanup with retained ID.
The opt-in live test covers create/read/update, exact-ID import with full attribute comparison, no-change plans, external drift repair, external deletion/recreation and final cleanup.
See [evidence](../../design/en/contract-progress.md) and [test execution](../../design/en/development.md).
Empty group-description creation/clearing, full account pagination, rule packet behavior, attachments and compute availability remain separate unresolved contracts.

Sources: official [create](https://iwinv.readme.io/reference/post_v1-security-groups), [detail](https://iwinv.readme.io/reference/get_v1-security-groups-id), [update](https://iwinv.readme.io/reference/put_v1-security-groups-id), [delete](https://iwinv.readme.io/reference/delete_v1-security-groups-id), and [Terraform create state rules](https://developer.hashicorp.com/terraform/plugin/framework/resources/create).
