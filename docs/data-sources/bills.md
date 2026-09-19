---
page_title: "iwinv_bills Data Source - iwinv"
subcategory: "Billing"
description: |-
  Reads complete bill summaries with date and price filters and sensitive results.
---

# iwinv_bills (Data Source)

[한국어](../ko/data-sources/bills.md) · [Development installation](../../design/en/development.md)

Reads complete bill summaries without modifying billing records. No Registry release yet. This data source calls the list endpoint;
it does not replace the unavailable detail endpoint's nested groups/items.

```hcl
data "iwinv_bills" "selected" {
  start_date    = "2026-01-01"
  end_date      = "2026-01-31"
  minimum_price = 0
}

output "bill_summaries" {
  value     = data.iwinv_bills.selected.bills
  sensitive = true
}
```

The dates are example filters. Omit all four filters to read all API-visible bill summaries.

| Attribute | Type | Meaning |
| --- | --- | --- |
| `start_date` | optional string | Inclusive **bill date** lower bound, YYYY-MM-DD. Not usage start. |
| `end_date` | optional string | Inclusive bill date upper bound, YYYY-MM-DD. |
| `minimum_price` | optional sensitive int64 | Inclusive lower bound for VAT-exclusive `price`. |
| `maximum_price` | optional sensitive int64 | Inclusive upper bound for VAT-exclusive `price`. |
| `bills` | computed sensitive list(object) | Complete rows sorted by exact bill ID. |

Omitted/null filters are unbounded. Empty dates, invalid calendar dates and inverted ranges fail before requests.
Zero and negative amount bounds are sent literally; unknown filters must resolve before Read and never silently become an account-wide query.
Returned rows must match requested filters. No currency conversion or minor-unit scaling is performed.

Each bill contains string `bill_id`, `usage_start`, `usage_end`, `bill_date`, `type`, `payment_status`, `name`, `currency`, `date_paid`,
and exact int64 `price`, `vat`, `payment_price`. Price excludes VAT; VAT and VAT-inclusive payment amount are observed, not calculated.
No arithmetic equality is enforced. Names/date strings remain literal, including an empty paid date; no timezone is inferred.
KRW has live evidence; other currencies, refunds/credits and unpaid variants remain unverified.
Payment-instrument information and invoice/tax-document links are excluded entirely. There is no raw-JSON escape hatch.

**The complete financial result remains in state and saved plans despite sensitive marking.** Root outputs need `sensitive = true`;
JSON output can reveal underlying values. Protect these artifacts and logs. Sensitivity is redaction, not encryption or removal.

The provider validates page-local counts and integer/canonical-string page metadata, traverses bounded pages and rejects duplicate IDs,
changed metadata, malformed rows and late errors without publishing a partial list. The exact first-page HTTP 400 `EMPTY_SET` means
an empty list; an ordinary successful empty array is also valid. Generic 400/404, permission errors and late `EMPTY_SET` are errors.
No snapshot token is available, so concurrent changes can cause undetectable omissions. No automatic retries, import or resource ownership apply.

T072 covers stable ordering/no-change plans, exact amounts above 2^53, sensitivity in plan/state, unmarked-output rejection, excluded
payment fields, unknown filters, zero/negative bounds and live read-only acceptance. This is summary access, not detail or payment support.

[Example](../../examples/data-sources/iwinv_bills/main.tf) · [Contracts and evidence](../../design/en/billing-contract.md) ·
[Current estimate](current_bill.md)

Source: [official bill list API](https://iwinv-common.readme.io/reference/get_new-endpoint-1).
