# DBMS lifecycle decisions

[한국어](../ko/dbms-lifecycle.md) · [Architecture](architecture.md) · 2026-09-19

`iwinv_db_instance` manages the public control-plane service and its entire IPv4 allowlist.
T062 covers the typed adapter; T063 covers Terraform Core and live STD Redis lifecycle acceptance.
Other engines, database access, backup/migration, billing termination and overall T038 remain unverified.
See the [resource guide](../../docs/resources/db_instance.md) for the executable schema.

## Identity, configuration and import

`service_idx` is preserved as an exact positive int64 decimal string, including values above 2^53.
Read selects this ID from a fully validated service list and rejects malformed/duplicate rows or changed pagination/count metadata.
The create receipt omits product ID and has a string domain; Read includes product ID and a label-to-domain object.
They have separate typed models. Readable models exclude arbitrary credentials and initial account history.

Read restores product, literal name/description, allowlist, status, tier/version and domain map. The default domain becomes `address`,
without inventing a port or deriving a login account. The API does not return the initial account name: `account_name` is optional
historical input for import but required for new creation. Adding, changing or removing it is a replacement request.
Import without it passes full readable-attribute verification and persisted re-import/no-change plan. Local timeouts are not imported.

Only allowlist changes have a public update endpoint. Other creation settings replace the entire service and destroy data.
Ordinary replacement requires a known different account name; imported history cannot establish whether a newly supplied name was used.
Unknown changed creation attributes conservatively plan replacement, and unknown account names cannot authorize that replacement.
Explicit taint/`-replace` and external deletion can bypass comparisons; use a fresh account and inspect the plan. The hosting account's
24-hour reuse restriction is not generalized to DBMS. DBMS reuse timing remains unknown.

## One owner for the allowed-IP set

The “add” endpoint replaces the entire list. `allowed_ips` is therefore a required nonempty Terraform set on the parent resource.
One authoritative owner avoids conflicting individual-IP resources and ambiguous standalone allowlist deletion semantics.
Ordering does not trigger a diff; external set changes are detected and restored. Empty PUTs were rejected and kept the prior set.
An observed empty set remains readable, but managed configuration must provide at least one supported IPv4 host.
CIDRs, IPv6 and null members are rejected before writes until verified; the provider does not fabricate a deny-all address.

PUT sends one JSON request. Its acknowledgement must contain exactly the requested set, followed by active Read with matching settings.
Timeout-only changes cause no write. Update errors, changed acknowledgements and waiter expiry keep prior state so refresh can reconcile
an uncertain remote outcome. No busy/error response becomes absence and no write is automatically replayed.

## Product ambiguity (C30)

A fresh unfiltered catalog contained 113 rows, including four `available` rows whose `product_id` was an empty string.
The adapter preserves these unselectable rows rather than inventing an ID or silently dropping them; null/missing IDs remain contract errors.
Creation rejects an empty ID. Catalog IDs also repeat across versions; preserve those rows and reject only duplicate ID/tier/version tuples.
The API exposes no version selector in POST. `engine_version` is observed, not configurable, and a catalog row does not prove provisioning eligibility.
`spec.type` is a tier, not an engine name. Prices/VAT, vCPU, memory and disk units remain outside the typed public model.
The internal catalog accepts documented engine/type filters; only the unfiltered and STD/redis combination were live-checked here.
DBMS catalog data sources remain unregistered.

The tested product was selected using the documented `type=STD&db=redis` query. Product names use abbreviated labels; the response does not separately echo an engine identifier. “STD Redis” in this evidence describes that filtered selection, not an independent engine or database-connectivity verification.

## Recovery and evidence

Create records the unambiguous returned ID before further receipt validation or readiness waits. Failed/malformed 201/202 contracts,
delayed visibility and timeout preserve identity. Read cannot forget a never-verified create solely because a successful list omits it.
A cleanup attempt addresses that known ID once; an already active service's validated absence can remove state.
Delete requires acknowledgement and subsequent exact-ID absence. This is distinct from independent billing confirmation.
The contradictory webmail absence contract remains separately unresolved under T056; it is not reused here.

T062 created two STD Redis services, verified literal/omitted descriptions, complete two-service visibility, a two-IP replacement,
peer preservation and acknowledged deletion/absence. T063 created four service identities across two coexisting resources, in-place
allowlist changes/drift repair, full import, persisted re-import without account history, fresh-account create-before-destroy,
external deletion/recreation and final cleanup. All six IDs were recorded privately and verified deleted; no pre-existing resource was mutated.

Synthetic tests additionally exercise malformed receipts with Core cleanup, unknown inputs, delayed creation, API/read/write errors,
waiter expiry, missing creation account, unsupported empty/IPv6/CIDR allowlists, replacement guards and explicit replacement plans.
They do not claim packet behavior, client access, backup, data migration, other engine variants or version selection.

Sources: [create](https://iwinv-dbms.readme.io/reference/클라우드-dbms-생성),
[allowlist](https://iwinv-dbms.readme.io/reference/접근-허용-ip-추가),
[delete](https://iwinv-dbms.readme.io/reference/클라우드-dbms-삭제),
[products](https://iwinv-dbms.readme.io/reference/클라우드-dbms-상품-조회).
