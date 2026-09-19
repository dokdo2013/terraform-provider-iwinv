---
page_title: "iwinv_current_bill Data Source - iwinv"
subcategory: "Billing"
description: |-
  Reads the current VAT-exclusive billing estimate with sensitive outputs.
---

# iwinv_current_bill (Data Source)

[한국어](../ko/data-sources/current_bill.md) · [Development installation](../../design/en/development.md) · [Complete schema reference](../guides/schema_reference.md#iwinv_current_bill-data-source)

Reads the current month-to-date estimate. This development data source does not perform payment or create a bill.
The estimate can change on every refresh; it is not a finalized charge or evidence that deleted infrastructure stopped billing.

```hcl
data "iwinv_current_bill" "current" {}

output "estimated_price" {
  value     = data.iwinv_current_bill.current.price
  sensitive = true
}
```

There are no input arguments. All five computed fields are sensitive:

| Attribute | Type | Meaning |
| --- | --- | --- |
| `bill_id` | string | Observed current bill ID. `BILL-live` was verified. |
| `start_date` | string | Literal observed start date. |
| `end_date` | string | Literal observed end date. |
| `price` | int64 | Estimated amount excluding VAT, in the returned currency; no unit scaling. |
| `currency` | string | Observed currency code; KRW was verified. |

The API returns an array; exactly one validated row is required. Zero or multiple rows produce a diagnostic rather than selecting an
arbitrary estimate. HTTP/authorization errors, malformed data, inconsistent count and unexpected pagination also fail the read.
Amounts retain exact integer precision, including values greater than 2^53. The provider does not calculate VAT or convert currencies.
The vendor's billing timezone and month boundary are unverified; date strings are not normalized to UTC.

**Sensitive values are still stored in Terraform state and saved plans.** They are redacted in normal human-facing output, but commands
such as `terraform show -json` expose underlying values. Protect state, plans and logs; `sensitive` is not encryption or exclusion.
Root outputs must explicitly set `sensitive = true`. No payment instruments, invoice links or tax-document links are exposed.

No import or remote ownership applies. T072 verifies Core sensitivity, exact money, cardinality/error behavior and read-only live use.
A no-change plan can be observed while the estimate remains stable; it is not a promise that future refreshes retain the same amount.
The separate `iwinv_bill` detail type is not implemented because its endpoint rejects current test access.

[Example](../../examples/data-sources/iwinv_current_bill/main.tf) · [Contract and limitations](../../design/en/billing-contract.md) ·
[Bill list](bills.md)

Source: [official current estimate API](https://iwinv-common.readme.io/reference/get_new-endpoint).
