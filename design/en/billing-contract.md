# Billing read contracts and remaining gates

[한국어](../ko/billing-contract.md) · [Architecture](architecture.md) · 2026-09-19

The internal read-only adapter covers `GET /v1/bill/live` and `GET /v1/bill`. No billing Terraform data source is registered yet.
The intended types remain `iwinv_current_bill`, `iwinv_bills` and `iwinv_bill`. Detail acceptance is blocked separately; do not substitute
list records for the detail endpoint's groups/items or claim that all billing operations are implemented.

## Public documentation and live distinctions (C32)

The live endpoint describes a month-to-date estimate excluding VAT. It returns an array with bill ID, start/end dates,
integer price and currency. The observed ID is the documented `BILL-live` constant. Preserve all rows rather than infer
one permanent invoice; a current estimate is not a settled charge or proof that deleted infrastructure stopped billing.

List records contain exact bill ID, usage dates, bill date, type/status/name, price excluding VAT, VAT, total payment price,
currency and date paid. The official CLI formats KRW amounts as won. Live observations were KRW integer amounts and matched
price plus VAT to payment price. The adapter performs no scaling, currency conversion, VAT calculation or equality enforcement;
discounts, refunds, credits and other currencies have not been validated. Preserve exact signed int64 values without float rounding.

The API dates observed were strings. Documented filter inputs use YYYY-MM-DD. Billing timezone and the exact boundary of
month-to-date calculation are not specified by these sources. Keep returned date text literal, without converting to UTC,
adding midnight/timezone offsets or assigning the request-signing timestamp's UTC semantics to billing dates.
Synthetic empty paid-date values are preserved; live fixtures contained paid dates, so unpaid variants need further evidence.

Documentation labels `count` as total count and page metadata as integers. Live list requests instead return **page-local count**
and string `page_no`/`page_size`. Two distinct size-one pages matched one size-two page. Validate count against row length and
page/size against the request; accept documented integers and observed canonical integer strings for page metadata only.
Traverse until a short/empty successful page, retain every exact ID, reject duplicates and enforce a maximum page count.
The API offers no snapshot token: inserts or changes during traversal can still cause omissions that cannot be detected from counts.

## Filters and empty results

Equal start/end dates and equal min/max amounts include their boundary values. A bill whose usage start differed from bill date
was included by its bill date and excluded by its usage start. The date filters therefore use **bill_date** in these observations.
Price filters compare the VAT-exclusive `price`, not total payment. Validate dates and bound ordering before requests and check
returned rows against all requested filters. Independent one-sided date/price bounds and negative-price filter bounds also matched the complete baseline; this does not establish real refund/credit records.

A filter with no matching bills returned HTTP 400 with exact `EMPTY_SET`; a page beyond the end returned HTTP 200 with an empty
array and page-local count zero. Only this list endpoint's exact first-page HTTP 400/`EMPTY_SET` is treated as an empty result.
A late-page error, including `EMPTY_SET`, fails the whole read without returning partial records. Generic 400/404, permission
errors and errors from `/live` or detail never become empty results. No request is automatically retried.

## Privacy and Terraform design

The list also returns payment-instrument information, invoice links and tax-document links. These are deliberately absent from
typed models, diagnostics, public fixtures and the proposed default Terraform schema. Do not follow or fetch those links.
Existing records are read-only; these APIs do not authorize payment, refund, account mutation or messages.

The future schema should mark financial outputs sensitive and explain that sensitive state remains stored by Terraform.
Do not expose raw JSON as a shortcut. One complete list owns no cloud resource, needs no import and should have stable ID ordering;
the current estimate can legitimately change on every refresh. Preserve literal labels and date strings. Additional detail fields
need explicit field-by-field privacy and identity decisions after the endpoint becomes available.

## Acceptance and blocked detail

T071: `TestAccBillingReads` passed in 18.07 seconds with Go race. It read the current estimate, traversed a multi-page list,
compared exact, independent one-sided and negative-bound filters with the complete baseline, and verified the empty-filter contract. The run made no writes.
Synthetic tests cover exact amounts above 2^53, excluded payment/URL fields, literal names, malformed/null fields, overflow/fractional
amounts, count/page mismatches, duplicate pages, traversal bounds, cancellation, filter validation and failure without partial results.
This is typed-adapter evidence only; Terraform Core state, sensitive-output behavior and data source registration remain pending.

Two different IDs obtained from the list and the documented `BILL-live` detail ID each returned HTTP 403 `CHECK_IP` (nested code 9)
from `GET /v1/bill/{bill_id}`, while the list and current estimate succeeded with the same local credentials. The reason for this
endpoint-specific denial is unresolved; it does not prove that the key or global IP allowlist is wrong. Do not retry unchanged
conditions, modify the allowlist or fabricate a successful detail fixture. Authenticated detail needs changed access conditions or
vendor clarification; no vendor message has been sent. T042 remains in progress because timezone, detailed unit semantics and
sensitive nested detail fields are not fully verified. Overall webmail cleanup T056 remains independently unresolved.

Sources: [current estimate](https://iwinv-common.readme.io/reference/get_new-endpoint),
[bill list](https://iwinv-common.readme.io/reference/get_new-endpoint-1),
[bill detail](https://iwinv-common.readme.io/reference/get_new-endpoint-1-3),
[official CLI billing guide](https://docs.iwinv.kr/developers/cli/commands/bill/).
