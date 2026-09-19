---
page_title: "iwinv_image Data Source - iwinv"
subcategory: "Compute"
description: |-
  Read the supported metadata for one exact image ID. No image is created, imported or owned.
---

# iwinv_image

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/image.md)

Read the supported metadata for one exact image ID. No image is created, imported or owned.

Development provider; no Registry release yet. Start with [provider setup](../index.md).

## Example Usage

```hcl
variable "image_id" {
  type = string
}

data "iwinv_image" "selected" {
  id = var.image_id
}

output "image_visibility" {
  value = data.iwinv_image.selected.visibility
}
```

## Argument Reference

`id` (required string): exact image ID from [images](images.md). It must be known and non-empty when Read executes. The provider accepts a single ID segment matching `[A-Za-z0-9_-][A-Za-z0-9_.-]*`; slashes, spaces and query fragments are rejected before an API request.

## Attribute Reference

| Attribute | Type | Meaning |
| --- | --- | --- |
| `id` | `string` | The configured exact ID, verified against the response. |
| `visibility` | `string` | Unmodified API visibility. |
| `image_type` | `string` | Unmodified API image type. |

`id` is required configuration; the other attributes are computed.

## Behavior and limitations

The response must contain exactly one row with the requested ID and non-empty detail fields. Missing, ambiguous or mismatched results are errors; there is no first-row or name fallback. This lookup neither returns image passwords nor selects a latest image. The limited public-image contract has live read/no-change-plan evidence. Private-image variants, image creation/deletion, architecture/OS specifications and instance compatibility have not been established by this data source.

Read-only; import does not apply. API errors do not become empty results.

[Example](../../examples/data-sources/iwinv_catalogs/main.tf) · [Evidence](../../design/en/contract-progress.md)

[Official API](https://iwinv.readme.io/reference/getv1imagesimageid)
