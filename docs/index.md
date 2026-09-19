---
page_title: "iwinv Provider"
description: |-
  Configure the independent community Terraform provider for iwinv.
---

# iwinv Provider

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/index.md)

Manage supported iwinv control-plane resources using Terraform configuration, references, import and drift detection. This is an independent community project, not an official SmileServ/iwinv product. The development provider currently implements eighteen data sources and seven resources. **No Registry version has been published.** Follow the [development installation guide](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/development.md) before running the example; the source address below identifies the provider but does not make it downloadable yet.

## Example Usage

Start with a read-only zone query. It does not provision a server or change your account.

```hcl
terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = {
      source = "dokdo2013/iwinv"
    }
  }
}

provider "iwinv" {}

data "iwinv_availability_zones" "available" {}

output "zone_ids" {
  value = data.iwinv_availability_zones.available.zone_ids
}
```

Provide `IWINV_ACCESS_KEY` and `IWINV_SECRET_KEY` through your private environment or secret manager. They are control-plane API credentials, not console login credentials, SSH keys, S3 keys or MCP OAuth tokens. Do not paste real keys into `.tf` files, examples, shell history or issue reports. Keep the key's allowed source IP consistent with the machine running Terraform. The provider uses the fixed HTTPS endpoint `https://api-kr.iwinv.kr`, verifies TLS and does not read CLI profiles or perform interactive login.

With a development override configured, run `terraform validate` and then `terraform plan`. Validation requires no API credentials; reading data during plan does. The override setup deliberately skips Registry `init`. [Release preparation](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/release-readiness.md) describes both unsigned filesystem-mirror installation and signature verification with a disposable test key followed by mirror installation. The latter checks the artifact signature separately; Terraform does not authenticate that GPG signature during mirror installation. Neither rehearsal establishes production signing-key trust or a signed Registry installation.

## Argument Reference

| Argument | Type | Configuration |
| --- | --- | --- |
| `access_key` | optional, sensitive string | Explicit value takes precedence over `IWINV_ACCESS_KEY`. Omitted/null uses that environment variable. |
| `secret_key` | optional, sensitive string | Explicit value takes precedence over `IWINV_SECRET_KEY`. Omitted/null uses that environment variable. |

Both resolved values must be known, contain a non-whitespace character and contain no line breaks when configuring the provider. Values are sent as supplied; surrounding spaces are not trimmed. An explicit empty or unknown value is an error, not a fallback to a different account. Each field resolves independently: when using an alias for another account, provide **both** values from the same intended account. Terraform's standard `alias` and `provider = iwinv.alias_name` select a provider configuration; no `region`, endpoint override, `profile` or `default_tags` setting is implemented.

`Sensitive` hides ordinary CLI display; it is not encryption and does not make arbitrary Terraform variables, outputs or saved plans safe to share. Prefer environment injection. Resource password handling and billing-state sensitivity are documented on their individual pages.

## Request timing and recovery

The client spaces request admission by one second **per provider configuration**, including reads and polling. This is a local pacing policy, not a verified account-wide quota. Aliases and separate Terraform processes have independent clients, even when they use the same account. Coordinate concurrent runs; adding aliases does not increase the account's allowed API rate.

Each HTTP attempt has a fixed 30-second timeout. Resource `timeouts` instead bound the whole operation, including request admission, HTTP calls and convergence polling: the current resources default to `read = "1m"` and `create`/`update`/`delete = "5m"`. A longer resource timeout does not lengthen the 30-second HTTP attempt. Data-source catalog traversal can make many requests and has no configurable `timeouts` block, so the HTTP timeout is not a total read deadline.

The shared transport does not automatically retry HTTP/API errors, redirects or uncertain writes. The [cache resource](resources/content_cache.md) has a narrowly scoped exception for a verified rejected-busy response; consult its recovery rules. A timeout or interrupted apply does not roll back a request already accepted by iwinv. Keep the state and inspect the exact resource before applying again; do not clear state or repeat creation simply because a request timed out.

## Find the right data source or resource

| Task | Start here |
| --- | --- |
| Inspect API-visible zones | [availability_zones](data-sources/availability_zones.md) |
| Choose an image by its exact ID | [images](data-sources/images.md), then [image](data-sources/image.md) |
| Choose a compute product by its exact ID | [instance_types](data-sources/instance_types.md), then [instance_type](data-sources/instance_type.md) |
| Reference an existing SSH key | [ssh_keys](data-sources/ssh_keys.md), then [ssh_key](data-sources/ssh_key.md) |
| Manage a security group and independent rules | [security_group](resources/security_group.md), [rule guide](guides/security_group_rules.md) |
| Manage hosted services | [webhosting](resources/webhosting.md), [db_instance](resources/db_instance.md), [content_cache](resources/content_cache.md), [shared_storage](resources/shared_storage.md) |
| Inspect financial summaries | [current_bill](data-sources/current_bill.md), [bills](data-sources/bills.md) |

Data sources read remote information and do not own or import objects. Resources can create, change and delete services and incur charges. Read the resource's import, replacement, secret and deletion constraints before applying; use dedicated test resources and check cleanup against their exact IDs.

Compute instances, attachments, webmail service/mailbox management, service-specific object/message operations and the broader roadmap remain incomplete. Catalog visibility does not prove creation eligibility, available capacity or compatibility. There is no AWS IAM/ARN/tag model hidden behind the AWS-familiar naming. The [capability ledger](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/inventory/implementation.json) and [verification record](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/contract-progress.md) distinguish implemented features from proposals and unresolved contracts.

## Troubleshooting

- **Invalid configuration:** check that both intended control-plane credentials are available to the Terraform process and are not explicitly empty or unknown. Do not print them to debug.
- **IP/authentication error:** check the API key's source-IP policy and the caller's system clock. An endpoint-specific denial can remain even when another endpoint works; do not treat it as an empty catalog.
- **No exact match:** choose an ID returned for the intended account, not a display name, converted AWS ID or list position. A failed detail lookup is not an instruction to create a replacement.
- **Contract/metadata error:** preserve a private diagnostic and report a sanitized summary. A malformed response or later-page error is intentionally not accepted as a partial list.

Official service documentation: [iwinv developer guide](https://docs.iwinv.kr/developers/cli/commands/account). Community bugs: [GitHub issues](https://github.com/dokdo2013/terraform-provider-iwinv/issues). Exclude keys, state, saved plans, raw account responses and financial data from reports.

See the [schema reference](guides/schema_reference.md) for all attributes, nested structures, input flags and secret flags.
