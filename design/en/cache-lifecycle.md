# Content-cache lifecycle decisions

[한국어](../ko/cache-lifecycle.md) · [Architecture](architecture.md) · 2026-09-19

The typed control-plane adapter now covers all five cache operations: products, service list, create, complete referrer-set replacement and delete.
T065 passes for two successive runs of two fresh `cache_lite` services each. **The Terraform resource is now registered with T066 evidence below; the catalog data source is now registered with T067 evidence.**

## Identity and observed configuration

Use exact positive int64 `service_idx` strings, without floating-point conversion. Read validates the complete list before selecting an ID;
malformed/duplicate rows, changed pagination/count metadata and API errors do not become absence. Extract creation identity before validating
remaining receipt fields. Preserve it across partial receipts, unverified 201/202 responses and later setup failure.

Read exposes product ID, literal alias/description, account `id`, status, IP, domain string, `spec.type` and the complete referrer list.
The create receipt omits product ID and cannot be decoded as the same required schema. Passwords, tenant API keys and arbitrary response
fields are excluded from readable models. Empty observed referrer arrays remain distinguishable from missing/null fields.
The initial account is readable, unlike DBMS; import can restore it instead of reconstructing it from DNS.

C31: `cache_lite` belongs to catalog type `SHARE`, but both new create receipts report `spec.type: SINGLE`, consistent with earlier captures.
Do not reject a known product because catalog and service labels disagree, or rewrite the observed service label to match a catalog assumption.
No shared/dedicated isolation guarantee follows from that inconsistent field. Product availability is not provisioning eligibility.
The catalog includes a coming-soon row with null `product_id`; preserve it as null and never invent a creation ID.
Unit-dependent disk/traffic/pricing fields remain excluded.

## Passwords and replacement

The official create schema advertises `ftppw`; live validation instead requires JSON `pw: {"FTP": "…"}`.
The adapter sends only this verified nesting and never serializes the initial password into its journal-input model.
Supported initial passwords are 7–20 non-space printable ASCII characters using at least two of letters, digits and symbols.
The account input is 6–12 ASCII alphanumeric characters. Literal Unicode names/descriptions and omitted-description behavior were rechecked.

The registered `iwinv_content_cache` resource reads an ephemeral `ftp_password_wo` from configuration, never state, and includes a positive
local version trigger for intentional new initial passwords. A trigger replaces the service; no password-rotation API has been verified.
Name, product, description and account changes also require replacement because there is no corresponding update endpoint.
The vendor explicitly documents irreversible deletion and a **24-hour account-name reuse restriction**. Ordinary replacement therefore
requires a known different account and fresh initial credentials. Taint/`-replace`, imported state, creation failure and external deletion
have synthetic Core tests, with live import/replacement/external-deletion coverage; a plan does not promise successful immediate recreation with the same account name.

## Complete referrer-set ownership and initialization

C22: create-time `allow_referer` is ignored. The adapter deliberately does not send it and does not claim that creation applied it.
The parent resource retains the new ID, verifies the service, then sets any desired nonempty referrer set with a separate PUT.
A failure between these steps must retain the parent identity rather than orphaning the billable service.

The PUT accepts a JSON `allow_referer` array and replaces the entire list. One parent resource should own the complete set; independent
per-referrer resources would conflict. Only canonical lowercase ASCII DNS hostnames have the initial supported input policy. Wildcards,
URLs/paths/ports, IDNs and IP literals need separate contract validation. Unsupported existing values must not be silently normalized away.
Empty clearing was rejected with HTTP 422. Empty initial configuration can omit the PUT, but changing a nonempty set to empty needs
explicit destructive replacement or a separately verified clearing API. Unknown sets at plan time need conservative replacement handling.

## Busy errors and retries

`active` does **not** establish that the write lock has cleared. The exact observed PUT/DELETE rejection is HTTP 404, `code: 0x1`,
`error_code: NOT_FOUND`, and identical message/result text identifying another operation in progress. Only that response on
`PUT /v1/cache/{canonical-positive-int64}/allow_referer` is classified as `cache_referrers_busy`; the same exact response on
`DELETE /v1/cache/{canonical-positive-int64}` becomes `cache_delete_busy`. The client exposes no server text.
Different text, status, method, service, malformed identity, generic 404, transport failures and unknown outcomes remain ordinary errors.
Classification itself performs no retry; the client and adapter each make one attempt.

T065 deliberately issued a second different set after the first accepted update. It received this busy rejection once; a successful exact-ID
Read proved that the entire previous set remained unchanged and the parent still existed. After a bounded delay, the same requested set was
accepted and observed. The peer service remained unchanged. This establishes a narrowly scoped path for a resource-level bounded retry:
only explicit busy rejection, only after exact-ID Read preserves the prior set, and only within the operation timeout. Stop if the row disappears,
Read fails, a different writer changes the set, the response differs or the deadline expires. Never replay an accepted write or an uncertain create.

A second run removed the initial test's 45-second scheduling delay and attempted deletion promptly after accepted updates.
The first parent deletion received the exact busy rejection once; Read confirmed the entire owned parent model was unchanged. A bounded
retry then received a valid delete acknowledgement and verified absence. The peer was preserved and subsequently deleted once.
Only a known busy rejection permits another attempt after reconciliation. Transport failures, other errors and accepted writes are never
replayed. No fixed delay is promoted into a readiness guarantee. This is adapter-level evidence; T066 below adds Core failure-state and recovery tests.

## Evidence and remaining gates

`TestAccCacheControlPlaneWrites` passed first in 89.81 seconds and then in 45.49 seconds with Go race: two concurrent owned services, nested password creation, literal and
omitted descriptions, complete list, one- and two-referrer replacement, the one busy rejection with unchanged Read, peer preservation and
acknowledged deletion plus exact-ID absence for all four service identities across the two runs. The second run additionally verified a
busy deletion with unchanged parent, with no artificial pre-delete delay. An independent console check showed an empty unfiltered cache list
after the first run; the second run has its own acknowledged API cleanup evidence.
The private journal contains intents, receipt IDs, attempted/acknowledged writes and cleanup observations; public fixtures use synthetic values.
Synthetic tests cover >2^53 IDs, nullable products, malformed/partial receipts/lists, credential exclusion, unsupported inputs and narrow busy
classification with no implicit retry. No pre-existing infrastructure was changed.

Pending: other products, wildcard/empty-clear-in-place semantics, tenant API credentials, content/FTP access, purge and billing.
These are part of the original scope. T037/T038/T039 and overall cleanup T056 are not completed by this adapter test; webmail remains separate.

## Registered Core resource (T066)

`TestAccContentCache` passed in 116.04 seconds with Terraform 1.14.2 and Go race. Five fresh identities covered two-service coexistence,
initial empty and nonempty referrers, stable-ID full-set updates, external drift restoration, readable-field import and persisted re-import
without password/version, no-change plans, empty-set create-before-destroy with a fresh account, password-version/description replacement,
and external deletion followed by fresh-account recreation. One PUT busy rejection was reconciled; no DELETE busy rejection occurred in
this run, so that branch uses the separate T065 live contract plus synthetic Core coverage. All five deletes were acknowledged and each
exact ID was absent; the independently refreshed console showed no service rows and no search filter. Existing infrastructure was unchanged.

Saved compressed plans and state artifacts were scanned for the generated ephemeral password. Full import comparison excludes only the
unrecoverable local password version. Synthetic Core tests additionally cover failed setup with ID cleanup, malformed receipts, delayed
creation, taint/explicit-replace plans, unknown whole-set replacement, ordering, missing/unsupported input, timeout/error state preservation,
and busy retries stopping on changed/missing parents or failed reads. Accepted/uncertain writes are not replayed.

The resource defaults the referrer set to empty. Clearing a nonempty set requires replacement; every ordinary replacement requires a known
different account and initial password. Timeouts default to 5m for writes and 1m for reads. The [resource guide](../../docs/resources/content_cache.md)
explains import, secret handling and recovery. This does not finish overall T038 or the separate unresolved webmail cleanup T056.

Sources: [create](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-생성),
[referrers](https://iwinv-cache.readme.io/reference/레퍼러-추가),
[delete](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-삭제),
[products](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-상품-조회),
[tenant content API](https://help.iwinv.kr/manual/938).
