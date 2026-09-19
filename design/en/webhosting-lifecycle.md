# Webhosting lifecycle decisions

[한국어](../ko/webhosting-lifecycle.md) · [Architecture](architecture.md) · 2026-09-19

**`iwinv_webhosting` is registered in the development provider. T060 passed for SHARE PHP 8.4.**
See the [resource guide](../../docs/resources/webhosting.md) for the executable schema and example.
Product/server catalog data sources, other product/version acceptance and overall T038 remain open.

## Replacement and account reuse

The public API exposes reads, creation and deletion, but no update endpoint. All remote creation attributes therefore require
replacement on change. Vendor documentation restricts reuse of a deleted account name for 24 hours (C29); this is a documented
restriction, not a live measurement of the deadline or successful recreation after that interval.

Ordinary replacement plans require a known different account name, a server selection and both initial passwords before deletion.
Unknown changed attributes conservatively require replacement; an unknown account name cannot authorize it. Timeout-only changes
are local and do not mutate the service. Synthetic Core tests cover these cases and missing replacement inputs.

Explicit `-replace`, taint and recreation after external deletion can bypass comparison with the prior attributes. Plan-only tests
confirm Core's replacement behavior without executing a same-name recreate. Users must choose a fresh account themselves in these
cases. The provider never retries a create for 24 hours or invents a password/update endpoint.

Live acceptance created a fresh account before destroying the previous account while preserving a second independent service.
This does not prove custom-domain reuse between two accounts, or migrate content, databases or DNS. The example uses `prevent_destroy`;
backup, migration and cutover remain deliberate user operations. Deletion destroys hosted data.

## Ownership and import

- The identity is an exact positive decimal `service_idx`, never converted through float64. Import is by ID, not account/name adoption.
- Read restores product, account, alias, description, firewall flag, status, IP and domain mappings.
- `server_id` is a historical creation selector absent from Read. It is optional for import, required for create/replacement, and any
  later addition/change/removal requires replacement. A stored selector does not assert current server placement.
- `custom_domains` owns the entire custom domain-to-folder map. `domains` observes all mappings. The documented generated
  `account_name.iwinv.net` domain is verified in the response and excluded from configured ownership; its absence is an error.
- External custom-map changes are drift. Reconcile configuration to adopt them or use a fresh account for replacement. Neither DNS
  nor folder creation is managed. Default `/` mappings were live-tested; arbitrary folder normalization remains unverified.
- Omit `server_id`, passwords and local `password_wo_version` when importing without creation history. Match all readable configuration.
  Full import comparison and persisted re-import followed by a no-change plan passed without these historical inputs.

## Password handling

Initial FTP/database passwords are distinct `Sensitive` + `WriteOnly` inputs read from `req.Config`, never the plan. Both are required
only for creation/replacement. The provider persists neither their value nor a hash. Changing only a write-only value cannot produce a
plan; a positive optional `password_wo_version` signals full account replacement, not in-place credential rotation.

Synthetic and live tests use ephemeral sensitive variables and inspect parsed plans, actual compressed saved-plan contents and state
for the supplied values. Import neither requires passwords nor synthesizes them. Readable response models exclude echoed credentials.
Supported inputs are distinct 7–20 character printable ASCII values from at least two character classes; this is not exhaustive live
verification of every vendor password boundary. See [HashiCorp guidance](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments).

## API and failure boundary

The typed adapter validates complete service lists, rejects duplicate/malformed rows or changed pagination/count metadata, and
preserves int64 identities. Names/descriptions are literal; explicit empty descriptions returned HTTP 422, so the resource omits its
empty default on create. Product catalogs exclude unverified prices, VAT and storage/traffic units.

Create sends one request, saves an unambiguous identity before validating remaining receipt fields, and waits for active Read with
matching managed attributes. HTTP 201/202 receipts with an identity preserve it while reporting their unverified success contract.
Malformed receipts, delayed visibility and waiter expiry retain the ID for reconciliation. A never-verified creation hidden from a
successful list is not silently forgotten: Read fails with identity retained; destroy addresses that known ID once.

A previously active service missing from a validated complete list is treated as externally deleted. An API error/404 never establishes
absence. Delete requires acknowledgement followed by exact-ID absence. The provider does not replay writes. These hosting observations
must not be generalized to webmail, whose console-present service was omitted by its successful API list.

## Verified scope and remaining gates

T059 covers adapter contracts. T060 covers Core create/read/no-op, full import, persisted re-import without history, fresh-account
replacement, peer preservation, external deletion/recreation, password artifact exclusion and four exact-ID cleanups in live acceptance.
Synthetic tests additionally cover failed-create ID cleanup, hidden creation recovery, unknown inputs, errors/timeouts, external domain
drift, missing replacement inputs and explicit replacement plans. CI uses synthetic fixtures only.

HTTP/FTP/database connectivity, migration, arbitrary product/version/domain combinations and billing termination remain unverified.
T056 remains failed for the separately tracked webmail cleanup; hosting cleanup does not close that gate. No signed Registry release exists.

Sources: [creation](https://iwinv-hosting.readme.io/reference/웹-호스팅-생성),
[deletion](https://iwinv-hosting.readme.io/reference/웹-호스팅-삭제),
[default domain](https://docs.iwinv.kr/service/web-hosting/web-hosting-guide/webhosting_domain/).
See [live evidence](contract-progress.md).
