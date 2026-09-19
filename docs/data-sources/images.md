---
page_title: "iwinv_images Data Source - iwinv"
subcategory: "Compute"
description: |-
  Read all API-visible image IDs, then deliberately select the image you intend to use.
---

# iwinv_images

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/images.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_images-data-source)

Read all API-visible image IDs, then deliberately select the image you intend to use.

Development provider; no Registry release yet. Start with [provider setup](../index.md).

## Example Usage

```hcl
data "iwinv_images" "available" {}

output "image_ids" {
  value = data.iwinv_images.available.ids
}
```

## Argument Reference

No service-specific arguments. Name filters, most_recent and automatic image selection are not implemented.

## Attribute Reference

| Attribute | Type | Meaning |
| --- | --- | --- |
| `ids` | `list(string)` | Exact image IDs across all pages, sorted lexically. |

All attributes are computed; do not assign values to them in configuration.

## Behavior and limitations

Pagination uses ten rows per page and stops at a short page. A full final page requires one more request. Page/count mismatches, duplicate IDs and later-page failures reject the entire read; a valid empty catalog remains empty. The maximum is 1,000 pages. There is no snapshot token, so concurrent catalog changes cannot be ruled out completely. List order expresses neither age nor recommendation. Use the exact-ID [image](image.md) data source for the currently supported detail fields. Public-image detail is live-tested; private-image response variants and richer metadata remain unverified. An image appearing here does not prove zone/product compatibility or permission to provision it.

Read-only; import does not apply. API errors do not become empty results.

[Example](../../examples/data-sources/iwinv_catalogs/main.tf) · [Evidence](../../design/en/contract-progress.md)

[Official API](https://iwinv.readme.io/reference/getv1images)
