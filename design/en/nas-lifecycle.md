# Shared-storage lifecycle decisions

[한국어](../ko/nas-lifecycle.md) · [Architecture](architecture.md) · 2026-09-19

The typed adapter covers five NAS control-plane operations: products, complete service list, create, whole permission-map replacement and delete.
`iwinv_shared_storage` is registered after T069 Core acceptance. Product lookup is registered as `iwinv_shared_storage_products` after T070 catalog acceptance.

## Evidence and identity

C19/C20/C21 describe absent response schemas and conflicting input documentation. Live requests use JSON, despite a multipart
header default. The documented allowlist array is actually an IPv4-host-to-permission object in both create and Read.
The update acknowledgement is a distinct array of objects with `ip` and `acl`; require exactly the requested map, with no duplicate IPs.
An acknowledgement does not replace a subsequent exact-ID Read.

Use exact positive int64 `service_idx`, without float rounding. Extract and journal returned identity before validating remaining
receipt fields. Read validates the entire service array; errors, malformed rows, duplicate IDs and pagination/count changes never
become absence. Read exposes product, alias, literal description, status, `spec.disk`, domain, `mount_info` and the full permission map.
The creation receipt omits mount information. Treat mount information as opaque observed text, never a command to execute.
Credentials and arbitrary fields are excluded from the typed model. Preserve nullable descriptions and observed empty maps.

A fresh `api_nas` probe reached active after a pending observation; active was first observed 7.12 seconds into polling.
It matched the requested 100 GB, literal Korean description and initial RO map, accepted a replacement containing RW/RO, and was
acknowledged deleted and absent. This is an observation, not a fixed readiness delay or latency guarantee.

## Capacity, creation and import

The official create API documents 100–2000 GB and an alphanumeric share name of 6–20 characters. The adapter sends integer `hdd`
and exact `sharename`. Catalog rows include an available `api_nas` with disk bounds and coming-soon entries with empty IDs, zero bounds
and null versions. Preserve these rows; empty IDs cannot create. Retain documented disk units, but exclude pricing until verified.
Product visibility does not imply provisioning eligibility; the available api_nas product has live lifecycle evidence at 100 and 200 GB.
No resize, rename or description-update endpoint is documented in this control-plane surface.

`sharename` is **absent from Read**. Do not parse the domain or mount string to invent creation history. The optional `share_name`
input is required on create and retained only for objects created by Terraform; import omits it. Adding/changing/removing that history,
product, name, description or capacity must explicitly replace the service. Import by exact service ID should restore every readable
setting and obtain a no-change plan without a share name. A new share name is the conservative replacement policy; reuse restrictions
for NAS remain unverified, and the hosting/cache 24-hour restriction is not generalized to NAS.

Deletion destroys the storage and is documented as irreversible. Replacement is not an in-place resize or data migration.
Use `prevent_destroy`, backups and an explicit migration plan for real data. `create_before_destroy` may establish a new share first
but does not copy files or reconfigure clients. Synthetic acceptance checks failed-create recovery and explicit taint/`-replace` plans; it does not verify same-name recreation.

## Permission-map ownership and waiters

One parent resource should own the **complete** nonempty map of canonical IPv4 hosts to exact `RO` or `RW`.
Updates replace the map, remove omitted hosts and change permissions for retained hosts. Do not split membership into independent
resources or allow multiple Terraform states to manage the same map. Empty PUTs were rejected with 422 in the earlier probe;
IPv6/CIDR and alternate permission values remain unsupported. The resource rejects these inputs before writes.
API read-back proves configuration, not NFS permission enforcement.

Creation must retain the ID while waiting for active status and matching readable settings. Pending/waiting are bounded by context;
unknown status and read failures require explicit diagnostics. A never-verified missing identity must survive refresh and remain
eligible for owned-ID cleanup. Updates wait for the complete requested map; failures keep prior state for reconciliation.
Delete requires an acknowledgement and validated exact-ID absence. No write is automatically retried, and cache-specific busy
classification must not be applied to NAS. A timeout, transport failure or generic 404 requires reconciliation before another write.

## Remaining acceptance

T069 covers Terraform schema/unknown values, import, no-change plans, drift and replacement. Failure and timeout paths have synthetic acceptance; live tests exercise successful outcomes.
NFS mounts, file access, tenant API authentication/operations, backups, snapshots, migration, other products and billing termination
remain outside the control-plane evidence. Overall T038 and the independent unresolved webmail cleanup T056 remain incomplete.

Sources: [create](https://iwinv-api-nas.readme.io/reference/공유-스토리지-생성),
[allowlist](https://iwinv-api-nas.readme.io/reference/접근-허용-ip-추가),
[delete](https://iwinv-api-nas.readme.io/reference/공유-스토리지-삭제),
[products](https://iwinv-api-nas.readme.io/reference/공유-스토리지-상품-조회).

## Typed adapter acceptance (T068)

`TestAccNASControlPlaneWrites` passed in 25.82 seconds with Go race. Two fresh api_nas services at 100 GB reached active; both literal and
omitted descriptions were checked. The complete permission map was replaced, then a retained host changed from RW to RO while the other
host was removed. The peer remained unchanged. Both services were acknowledged deleted and absent by exact ID; together with the earlier
probe this stage cleaned three new identities. The API NAS console independently showed an empty unfiltered list afterward.
Synthetic checks preserve IDs after bad receipts, reject malformed/partial lists and wrong permission acknowledgements, and prohibit
invalid inputs or automatic write replay. Catalog bounds/version/nullability are verified separately from provisioning eligibility.
This stage was adapter-only acceptance; subsequent Terraform acceptance is recorded below.

## Terraform acceptance (T069)

The resource passed live Core acceptance in 82.29 seconds: two 100 GB services, permission update/drift, full import,
persisted import/no-op and update without share history, fresh-share replacement at 200 GB, external deletion/recreation,
and four acknowledged deletions with exact-ID absence. The independently refreshed console was empty.
Synthetic tests cover unknown values, invalid inputs, failures/timeouts, delayed visibility and ID retention, and exact-parent checks
before PUT. No writes are replayed. Taint/`-replace` are plan-only tests; no same-share reuse guarantee is made.
See the [user guide](../../docs/resources/shared_storage.md) and [detailed evidence](contract-progress.md).
