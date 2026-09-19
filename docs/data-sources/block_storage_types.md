---
page_title: "iwinv_block_storage_types Data Source - iwinv"
subcategory: "Storage"
description: |-
  Read block-storage type capacity bounds and nullable availability zones.
---

# iwinv_block_storage_types

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/data-sources/block_storage_types.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_block_storage_types-data-source)

Reads the disk-type catalog without provisioning storage. Development provider; no Registry release yet.

```hcl
data "iwinv_block_storage_types" "all" {}

data "iwinv_block_storage_types" "ssd" {
  type = "ssd"
}
```

`type` is an optional exact string filter. Omit it or use null for the complete catalog. Empty strings are rejected before a request;
unknown values defer the read until apply rather than falling back to an unfiltered read. No hardcoded type enum or implicit default
is chosen. Unsupported type codes can return API errors: the live unknown-code probe returned HTTP 400 `CHECK_PARAM`, not an empty result.

`types` is a computed list of objects sorted lexically by disk type:

| Attribute | Type | Meaning |
| --- | --- | --- |
| `type` | string | Exact API disk type; not an AWS volume type mapping. |
| `minimum_size_gb` | int64 | Minimum capacity in documented GB, without binary-unit conversion. |
| `maximum_size_gb` | int64 | Maximum capacity in documented GB; does not promise resize support. |
| `availability_zones` | nullable list(string) | Exact zone codes, sorted lexically. Null and empty lists remain distinct. |

The observed catalog contains SSD bounds of 10–2000 GB with null zones, and SATA bounds of 10–20000 GB with one zone.
The provider neither interprets null as all zones nor substitutes the account's zone catalog. Underscores/hyphens in zone codes are
preserved. The public example's SATA zone differs from the live result, so use the current response rather than copying a documentation
example. Catalog visibility alone does not establish creation permission, instance compatibility, attach/detach safety or pricing.
Block-storage lifecycle remains unimplemented while compute provisioning is restricted in the test account.

One unpaginated HTTP 200 response must contain an array and its exact count. Missing/null type or bounds, negative or reversed bounds,
non-integer/overflowing numbers, missing or malformed zones, empty/null/duplicate zone entries, duplicate disk types, filter mismatches,
unexpected metadata and API errors fail the entire read. A valid successful empty array remains an empty list; HTTP errors do not.
No import or remote ownership applies. Arbitrary response fields are excluded.

T074 combines adapter unit tests and Terraform Core tests for strict decoding, exact integers beyond 2^53, null/empty preservation, type/zone order changes, filters, errors and
unknown values. Live reads cover the complete catalog plus SSD and SATA filters followed by a no-change plan;
a separate read-only probe confirms unsupported-filter failure. This does not verify volume creation, attachment, resize or deletion.

[Example](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/examples/data-sources/iwinv_block_storage_types/main.tf) · [Evidence](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/contract-progress.md)

Sources: [official API](https://iwinv.readme.io/reference/getv1blockstoragestypes),
[official CLI commands](https://docs.iwinv.kr/developers/cli/commands/block-storages).
