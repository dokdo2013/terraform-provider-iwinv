# Feature guide behavior review

[한국어](../ko/guide-review.md) · [Documentation policy](documentation.md)

On 2026-09-19, the English and Korean feature guides for the 18 registered data sources and seven resources were reviewed against the implementation at `fb654c3406e697b0b7d0a0289ff3e3cbbbcec9a8`. The review covered the topics below: supported inputs, identity, collection ownership, update/replacement, import, empty results and failure recovery. This is a manual, scoped review, not a proof of every statement or a new live acceptance run. The correction commit containing this document changes guidance and one diagnostic, not validators, schemas or remote behavior.

## Corrections

- The security-group detail guide contained the list endpoint's pagination paragraph. It now describes a single exact-ID detail request and its not-found diagnostic. The list guide no longer describes an input ID it does not accept. Both languages link to the appropriate alternative.
- Webhosting password guidance and its diagnostic now explicitly exclude spaces, matching `validHostingPassword` and the existing `Has space1!` rejection test. The supported 7–20 character range and two-character-class requirement are unchanged.
- Webhosting's local `password_wo_version` is documented as a positive integer (`int64`), rather than an unrestricted number. It still triggers full account replacement; it is not in-place password rotation.

## Reviewed topics and implementation references

All names below have the `iwinv_` prefix. Each entry covers both `docs/` and the corresponding `docs/ko/` feature page. References identify the implementation checked; they do not imply that every vendor variant has live evidence.

| Feature pages | Topics compared | Implementation |
| --- | --- | --- |
| `availability_zones` | Exact IDs/status, sorted aligned outputs, count/duplicate validation, visibility does not establish provisioning permission. | [Zone adapter](../../internal/services/compute/zones.go), [projection](../../internal/provider/availability_zones_data_source.go) |
| `images`, `image` | Ten-row short-page traversal versus exact detail lookup; ID syntax, no latest/private-image assumptions. | [Catalog adapter](../../internal/services/compute/catalogs.go), [data sources](../../internal/provider/catalog_data_source.go) |
| `instance_types`, `instance_type` | Total-based traversal, exact flavor IDs including dots, one matching detail, no invented CPU/price/zone fields. | [Catalog adapter](../../internal/services/compute/catalogs.go), [data sources](../../internal/provider/catalog_data_source.go) |
| `ssh_keys`, `ssh_key` | Complete list validation before exact-ID selection; empty names, duplicate IDs, no key material. | [SSH adapter](../../internal/services/compute/ssh_keys.go), [data sources](../../internal/provider/ssh_key_data_source.go) |
| `webhosting_products`, `webhosting_servers` | SHARE/SINGLE filters, sorted product/server IDs, PHP labels, exact decimal server IDs and unreadable creation history. | [Catalog adapter](../../internal/services/hosted/webhosting_catalog.go), [data sources](../../internal/provider/webhosting_catalog_data_source.go) |
| `db_instance_products` | Engine/tier filter inputs, empty and repeated product IDs across versions, no creation-version selector. | [DBMS adapter](../../internal/services/hosted/dbms.go), [projection](../../internal/provider/db_instance_products_data_source.go) |
| `content_cache_products` | Null versus empty IDs, full rows and filter matching, catalog type is not an isolation guarantee. | [Cache adapter](../../internal/services/hosted/cache.go), [projection](../../internal/provider/content_cache_products_data_source.go) |
| `shared_storage_products` | No inputs, nullable version, exact capacity bounds, empty-ID rows and replacement rather than resize. | [NAS adapter](../../internal/services/hosted/nas.go), [projection](../../internal/provider/shared_storage_products_data_source.go) |
| `webmail_products` | Empty-ID catalog rows, stable ordering and no service/mailbox lifecycle support. | [Webmail adapter](../../internal/services/hosted/webmail_catalog.go), [projection](../../internal/provider/webmail_products_data_source.go) |
| `block_storage_types` | Optional exact filter, null/empty zones, int64 bounds, strict response validation and no volume lifecycle. | [Storage adapter](../../internal/services/storage/types.go), [data source](../../internal/provider/block_storage_types_data_source.go) |
| `current_bill`, `bills` | Single estimate versus paginated summaries, sensitive outputs, literal money, null/unknown filters and endpoint-specific first-page EMPTY_SET. | [Billing adapter](../../internal/services/billing/billing.go), [data sources](../../internal/provider/billing_data_source.go) |
| `security_groups`, `security_group` data sources | List versus detail endpoint, exact IDs, once-decoded descriptions, no rules/attachments ownership. | [Group adapter](../../internal/services/network/groups.go), [data sources](../../internal/provider/security_group_data_source.go) |
| `security_group` resource | Defaults/lengths, in-place attributes, import identity, retained failed-write state and verified empty detail contract. | [Resource](../../internal/provider/security_group_resource.go), [adapter](../../internal/services/network/groups.go) |
| `security_group_ingress_rule`, `security_group_egress_rule` | Composite identity, protocol/port/CIDR inputs, fixed direction, description-clear and parent replacement. | [Resources](../../internal/provider/security_group_rule_resource.go), [adapter](../../internal/services/network/rules.go) |
| `webhosting` | Initial write-only credentials, full account replacement, fresh account name, historical server input, domains and failed-create recovery. | [Resource](../../internal/provider/webhosting_resource.go), [adapter](../../internal/services/hosted/webhosting.go) |
| `db_instance` | Nonempty authoritative IPv4 set, historical account input, in-place allowlist versus destructive replacement, no engine-version selector. | [Resource](../../internal/provider/db_instance_resource.go), [adapter](../../internal/services/hosted/dbms.go) |
| `content_cache` | Authoritative referrers, empty-clear/unknown replacement, write-only password trigger, narrowly classified busy retry and retained identity. | [Resource](../../internal/provider/content_cache_resource.go), [adapter](../../internal/services/hosted/cache.go) |
| `shared_storage` | Nonempty IPv4→RO/RW map, share-name history, capacity replacement, pre-update parent check and no automatic write retry. | [Resource](../../internal/provider/shared_storage_resource.go), [adapter](../../internal/services/hosted/nas.go) |

## Evidence boundaries and remaining work

The existing adapter and Terraform Core tests provide executable evidence for these behaviors; the manual review does not replace them. In particular, [webhosting validation tests](../../internal/services/hosted/webhosting_test.go) already reject a password containing a space before any request and check that diagnostics do not contain either password. [Security-group data-source tests](../../internal/provider/security_group_data_source_test.go) distinguish list/detail, unknown input, invalid/missing IDs and API failures. No new cloud resource was needed for these corrections.

T052 remains `in_progress`. The provider introduction and shared guides need a separate narrative review; published Registry navigation, version-specific destinations and a bilingual documentation site remain release work. The six changed feature pages were rechecked in the official Registry body preview: their headings and correction text were visible, frontmatter was hidden, and all six tables rendered. Only those six [page records](../inventory/doc-preview.json) were updated with the observed content hashes and recheck date; the other records retain their earlier evidence. T080 continues to cover structural/schema/example checks only.

Control-plane acceptance does not establish packet filtering, database/file access, pricing or billing termination. Existing product-specific live limits, the unresolved webmail cleanup and the unconsented OAuth registration cleanup remain unchanged. Compute adapters are still unregistered candidates; these guide corrections do not add a usable instance resource or a Registry release.
