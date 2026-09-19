# Full capability coverage

[한국어](../ko/coverage.md) · [Index](../../README.en.md) · Revision: 2

Goal: account for every official iwinv API/CLI capability and support all controllable remote resources.
Coverage does not mean turning every CLI command into a persistent resource. Each entry is classified as
managed resource (R), data source (D), action (A), ephemeral resource (E), local tool (L), or discovery gap (G).
**Seven data sources are implemented and live-verified: `iwinv_availability_zones`, `iwinv_images`, `iwinv_image`, `iwinv_instance_types`, `iwinv_instance_type`, `iwinv_ssh_keys`, and `iwinv_ssh_key`. Managed resources remain proposals.**
See the [development guide](development.md) and [implementation ledger](../inventory/implementation.json).

## Control-plane inventory

The [machine-readable inventory](../inventory/api.json) lists each documented method/path, request fields,
content type, response status and source. Reviewed totals: 36 IaaS + 4 common + 5 hosting + 5 cache +
5 DBMS + 5 NAS + 6 webmail = 66 HTTP operations across 75 pages.

| Capability | Proposed Terraform surface | Class | Blocking contracts |
| --- | --- | --- | --- |
| Zones | `iwinv_availability_zones` | D | Zone status/eligibility and product compatibility |
| Flavors | `iwinv_instance_type(s)` | D | ID lookup, filters and units |
| Images | `iwinv_image(s)` | D | Public/private shape; deterministic single match |
| Instances | `iwinv_instance(s)` lookup; `iwinv_instance` resource | R/D | C01–C13, C17 |
| Power/reboot/rebuild | Separate Action candidates; power-state resource only after ownership ADR | A/G | Disruption, replay, state reconciliation, billing |
| Remote console | `iwinv_instance_console` candidate | E | Expiry and exposure; no normal-state URL |
| User scripts | `iwinv_user_script(s)` lookup; future management | D/G | Console-only creation; C18 |
| Block storage types | `iwinv_block_storage_types` | D | Allowed size/type per product |
| Block storage | `iwinv_block_storage(s)` lookup and resource | R/D | C14–C15, attached-create ownership |
| Block attachment | `iwinv_block_storage_attachment` candidate | R/G | Must not compete with volume connection owner |
| Security groups | `iwinv_security_group(s)` lookup and resource | R/D | Container Read, ICMP and deletion constraints |
| Security rules | `iwinv_security_group_ingress_rule`, `iwinv_security_group_egress_rule` | R/D | C16, numeric vs string ID, no inline ownership |
| Group membership | `iwinv_security_group_attachment` | R/D | Multiplicity, replacement vs additive attachment |
| Bills/payments | `iwinv_bill(s)`, `iwinv_current_bill` | D | Sensitive billing fields, currency/VAT/time zone |
| SSH keys | `iwinv_ssh_key(s)` lookup; future key resource | D/G | Reviewed API only lists keys |
| Web hosting | `iwinv_web_hosting`, product/server lookups | R/D | Password state exposure, no update contract, C19–C20 |
| Content cache | `iwinv_content_cache`, product lookup, referrer set | R/D/G | C19–C22; cache purge is a separate service API |
| Cloud DBMS | `iwinv_db_instance`, product lookup, allowlist set | R/D/G | C19–C21; destructive replacement and backup semantics |
| API NAS | `iwinv_shared_storage`, product lookup, allowlist set | R/D/G | C19–C21; data retention and protocol-specific access |
| Webmail service/accounts | `iwinv_webmail`, `iwinv_webmail_account`, product lookup | R/D/G | C19–C20, C23; mailbox Read and password semantics |

Names with `(s)` denote separate singular/plural candidates, not literal identifiers.
All 66 operations belong to these groups; missing update/read operations are documented gaps rather than fabricated APIs.

## Service APIs and remaining discovery

| Surface and source | Intended coverage | Status |
| --- | --- | --- |
| [Object storage API](https://help.iwinv.kr/manual/736) and official object CLI | Bucket/object lookup, objects, verified bucket settings (R/D), copy/move (A), presign (E) | Compatibility matrix required for endpoint style, signing, multipart, pagination, ACL/versioning/lifecycle; no bucket-create claim from CLI list commands |
| [NAS service API](https://help.iwinv.kr/manual/763) | Separate service-side operations from NAS subscription lifecycle | G: enumerate operations/auth and map to R/D/A |
| [Cache service API](https://help.iwinv.kr/manual/938) | Configuration/query and purge as appropriate | G: enumerate contracts; subscription API alone is insufficient |
| [SMS](https://docs.iwinv.kr/developers/api/Message_api/) | Send SMS/LMS/MMS/international (A), history/balance (D), reservation operations if supported | Endpoint URLs collected; classify semantic reads even if POST; verify quotas, scheduling timezone, payload bytes and PII handling |
| [Alimtalk](https://docs.iwinv.kr/developers/api/kakao_api/) | Templates (R/D), send/cancel (A), history/balance (D) | Template review-state constraints and no-replay semantics required |
| [MCP](https://docs.iwinv.kr/developers/mcp/) | Compare authenticated tools with inventory | G: tools/list not inspected; API parity not assumed |
| SDK/other products | Discover newly documented services, including any API outside the central developer index | G: public index is not proof of completeness |

All remote service capabilities remain within the long-term discovery scope. Sending messages, object transfer and
similar data-plane actions require explicit invocation semantics and will not run on refresh or routine apply by surprise.
Do not introduce a generic arbitrary-HTTP escape hatch as a substitute for documented coverage.

## CLI disposition

[CLI evidence](../inventory/surfaces.json) records public command references, not installed-binary verification.

| CLI group | Provider disposition |
| --- | --- |
| instances, block-storages, flavors, images, zones, ssh-keys, user-script | Map to the R/D/A/E groups above |
| bill, netstat | Read-only billing/traffic candidates; investigate API equivalents for netstat |
| object-storage auth/ls/ll/cp/mv/rm/presign | Auth is configuration; list is D; object lifecycle R; operations A; presign E |
| account, login, logout | Local authentication/profile management (L); Provider uses explicit config/env/aliases, never exports secrets |
| completion, help, theme, install/reinstall/update/uninstall | Local CLI behavior (L), not missing cloud coverage |
| undocumented admin or unknown subcommands | G; obtain supported contract, never infer from a help label |

## Coverage accounting

Track discovered, classified, contract-verified, implemented, acceptance-tested and released separately.
Numerator for support is released-and-tested capabilities, never number of documentation pages fetched.
Record explicit G blockers in the denominator; report local-tool exclusions separately.
Each new service family requires discovery, lifecycle/ownership ADR, credentials, error taxonomy, tests and both languages.
Refresh inventories manually and review diffs before updating coverage claims.
