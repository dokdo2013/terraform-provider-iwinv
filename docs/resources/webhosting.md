---
page_title: "iwinv_webhosting Resource - iwinv"
subcategory: "Hosting"
description: |-
  Manages one webhosting account with initial write-only credentials and explicit replacement constraints.
---

# iwinv_webhosting (Resource)

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/resources/webhosting.md) · [Development installation](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/development.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_webhosting-resource)

Development support for the public hosting control plane. No Registry release exists yet.
The live-tested scope is shared hosting with PHP 8.4, default/custom domains and explicit web-firewall Y/N.
Other products/versions, data-plane access and billing termination require further verification.
This resource owns one account's readable creation attributes and its complete custom domain map.
It does not manage website files, database contents, DNS, TLS certificates, traffic resets or console-only changes.

## Example

Use [the complete example](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/examples/resources/iwinv_webhosting/main.tf) with explicit product/server catalog IDs,
a fresh 6–12 letter account name, and two distinct ephemeral password variables. Review the [product](../data-sources/webhosting_products.md) and [server](../data-sources/webhosting_servers.md) catalogs before choosing IDs.
The account name is not the decimal service ID.

```hcl
variable "product_id" { type = string }
variable "server_id" { type = string }
variable "account_name" { type = string }

variable "ftp_password" {
  type      = string
  sensitive = true
  ephemeral = true
}
variable "database_password" {
  type      = string
  sensitive = true
  ephemeral = true
}
resource "iwinv_webhosting" "example" {
  product_id           = var.product_id
  server_id            = var.server_id
  account_name         = var.account_name
  name                 = "tf-example-hosting"
  ftp_password_wo      = var.ftp_password
  database_password_wo = var.database_password
  password_wo_version  = 1

  lifecycle { prevent_destroy = true }
}
```

Supply secret variables through your secret manager or process environment; do not put values in Git, ordinary outputs or command history.
Both write-only arguments are required to create or replace an account, but are optional for import and ongoing reads.
Passwords must differ and each contain 7–20 non-space printable ASCII characters from at least two of letters, digits and symbols.
This is the provider's supported input subset, not exhaustive live verification of every vendor password boundary.

## Changes and data loss

The public API has no hosting update endpoint. A remote configuration change replaces the entire account, destroying its hosted data.
The vendor documents a 24-hour restriction on reusing a deleted account name (C29). Ordinary replacement plans therefore require a
known, different `account_name`, a server selection and both initial passwords. Merely changing the alias can require replacement.
Local timeout changes alone do not write to the API.

Use a new account, back up/migrate content separately, and review DNS cutover before deletion. `create_before_destroy = true` can create
a fresh account before deleting the old one, but does not guarantee custom-domain reuse or move data/DNS for you.
The example uses `prevent_destroy` to block accidental deletion. Removing that protection requires an intentional configuration edit.
Removing the resource block also removes this guard; console/API deletion remains possible. [Scope](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle#prevent_destroy).

**Explicit `-replace`, taint and external deletion are separate Core behaviors.** The provider cannot detect/block every such replacement
through attribute comparison. A same-name recreation may fail after deletion; inspect the plan and use a fresh account name.
There is no 24-hour retry loop or invented in-place update.

Changing only a write-only password does not produce a plan. `password_wo_version` is an optional positive integer local trigger: changing it
requires full replacement with a fresh account name, not in-place password rotation. Neither password nor a password hash is persisted
by the provider. Use ephemeral variables so the configuration itself does not embed a secret literal in stored artifacts.

## Schema

| Attribute | Type | Behavior |
| --- | --- | --- |
| `product_id` | required string | Exact catalog product ID; replacement on change. |
| `account_name` | required string | 6–12 ASCII letters; replacement on change. |
| `name` | required string | 4–32 Unicode code points; replacement on change. |
| `server_id` | optional string | Canonical positive decimal catalog idx; required for new creation, absent from Read. Historical input; any change requires replacement. |
| `description` | optional + computed string | Default empty, maximum 50 code points. Empty is omitted on create because an explicit API empty string returns 422. |
| `web_firewall_enabled` | optional + computed bool | Default true; explicit API Y/N. No packet-effect claim. |
| `custom_domains` | optional + computed map(string) | Default empty. Complete custom domain→folder map, excluding `account_name.iwinv.net`; changes replace. |
| `ftp_password_wo`, `database_password_wo` | optional write-only sensitive string | Initial passwords; required for creation, never restored by import. |
| `password_wo_version` | optional int64 | Positive integer local replacement trigger; no remote counterpart. |
| `id` | computed string | Exact decimal service_idx, preserved without float64 conversion. |
| `status`, `ip_address` | computed string | Observed control-plane status and IP. |
| `default_domain` | computed string | Expected account-name subdomain, verified present in the response. |
| `domains` | computed map(string) | All observed mappings, including the service-managed default domain. |

Name/description text is preserved verbatim, including literal HTML entities. No trimming or Unicode normalization is applied.
Read separates the documented default subdomain from custom mappings. Unexpected absence of that default is a contract error, not a guessed ownership decision.
External additions/removals/changes to custom mappings are drift and can require replacement; matching configuration can instead adopt a deliberate external change.
Folder names are sent as supplied. Folder creation, normalization beyond verified `/`, and availability of arbitrary domain combinations are not promised.

`timeouts`: create/update/delete default `5m`, read `1m`; all must be positive duration strings.
`active` means control-plane state only, not successful HTTP/FTP/database access.

## Import

Import an exact decimal service ID with configuration matching the readable remote fields:

```sh
terraform import iwinv_webhosting.example 123456789
terraform plan
```

The number above is a synthetic placeholder. Omit `server_id`, both passwords and `password_wo_version` when importing without creation history.
They are not recoverable from Read. Match the observed description, firewall flag and custom-domain map; defaults are desired configuration, not imported facts.
Adding historical/local inputs later is a configuration change that may require replacement. Local timeouts are not imported.
Never manage one account from two states simultaneously.

## Failure recovery and verification

Creation sends one request and stores an unambiguous ID before validating the remaining receipt or waiting for active, matching Read results.
Errors retain that ID; Terraform may taint it. A later empty list does not erase a never-verified creation identity. Reconcile it before another apply.
Destroy addresses a known unverified create once even if its pre-delete list is empty. For a previously active service, validated absence can remove state.
HTTP/API errors, malformed lists and changed pagination metadata are never absence. Writes are never automatically replayed.
An acknowledged deletion must be followed by exact-ID absence; it is not independent billing confirmation.
Do not generalize this hosting contract to webmail, whose API can omit an existing service.

T060 covers synthetic Core failure/drift/replacement tests and live two-service create/import/no-op, fresh-account create-before-destroy,
persisted re-import without historical inputs, external deletion and cleanup. Saved plan archives and state were checked for ephemeral passwords.
Explicit taint/`-replace` tests inspect the plan only, without performing unsafe same-name recreation.
See [evidence](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/contract-progress.md) and [lifecycle decisions](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/webhosting-lifecycle.md).

Sources: [creation](https://iwinv-hosting.readme.io/reference/웹-호스팅-생성),
[deletion](https://iwinv-hosting.readme.io/reference/웹-호스팅-삭제),
[default subdomains](https://docs.iwinv.kr/service/web-hosting/web-hosting-guide/webhosting_domain/),
and [write-only arguments](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments).
