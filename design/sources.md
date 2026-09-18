# Evidence register / 근거 대장

Observed / 조사일: 2026-09-18. Public documentation only / 공개 문서만 조회.

## Evidence levels / 근거 수준

- `documentation`: published prose or embedded OpenAPI read successfully / 문서 또는 포함된 명세 확인.
- `unverified`: discovery or contract unresolved / 추가 조사 또는 계약 미확정.
- `live_verified`: authenticated operation tested with retained sanitized evidence / 실호출 증거 필요.
- `implemented`: provider code exists / Provider 구현 여부. Documentation does not imply this flag.

The initial inventory has no live-verified or implemented capabilities.
This project records endpoint/schema facts and links, not a copy of the vendor's documentation or SDK.
문서 추출은 구현·실환경 성공을 의미하지 않으며 원문 전체를 재배포하지 않습니다.

## iwinv sources

| Source / 출처 | Use / 용도 |
| --- | --- |
| [Developer API index](https://docs.iwinv.kr/developers/api/) | Published service families and Beta wording |
| [IaaS index](https://iwinv.readme.io/llms.txt) | Compute, block storage, security groups |
| [Common index](https://iwinv-common.readme.io/llms.txt) | Billing and SSH key lookup |
| [Hosting index](https://iwinv-hosting.readme.io/llms.txt) | Hosting lifecycle and catalogs |
| [Cache index](https://iwinv-cache.readme.io/llms.txt) | Cache lifecycle/referrers |
| [DBMS index](https://iwinv-dbms.readme.io/llms.txt) | DBMS lifecycle and allowlists |
| [NAS index](https://iwinv-api-nas.readme.io/llms.txt) | Shared storage lifecycle and allowlists |
| [Webmail index](https://iwinv-webmail.readme.io/llms.txt) | Service/account lifecycle |
| [Request authentication](https://iwinv-common.readme.io/reference/api-request) | Signature, clock and quota |
| [Response envelope](https://iwinv-common.readme.io/reference/api-response) | HTTP/business error distinctions |
| [API key policy](https://docs.iwinv.kr/developers/api/api-key-management/) | Keys and IP allowlist |
| [Instance fields](https://api-kr.iwinv.kr/fields/v1/instances) | Bitmask and sensitive field exclusions |
| [CLI](https://docs.iwinv.kr/developers/cli/) | Command surface discovery |
| [CLI object storage](https://docs.iwinv.kr/developers/cli/commands/object-storage/) | Object operations and 15-minute presign |
| [Object service API](https://help.iwinv.kr/manual/736) | Linked source; detailed compatibility audit pending |
| [NAS service API](https://help.iwinv.kr/manual/763) | Linked source; operation audit pending |
| [Cache service API](https://help.iwinv.kr/manual/938) | Linked source; operation audit pending |
| [SMS](https://docs.iwinv.kr/developers/api/Message_api/) | Service-specific authentication and message operations |
| [Alimtalk](https://docs.iwinv.kr/developers/api/kakao_api/) | Templates, review constraints and operations |
| [MCP](https://docs.iwinv.kr/developers/mcp/) | Official client connection surface |
| [MCP public metadata](https://mcp.iwinv.kr/.well-known/oauth-protected-resource) | OAuth resource and authorization-server metadata |

Each extracted API operation has its exact source URL in [api.json](inventory/api.json).
CLI and messaging URL observations are in [surfaces.json](inventory/surfaces.json).
Indexes can omit operations; successful fetch is not completeness proof. Some plain requests received HTTP 403;
public pages were retrieved using an ordinary browser User-Agent, without account authentication or access-control bypass.

## Terraform design references

| Source | Design use |
| --- | --- |
| [Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework) | New-provider implementation baseline |
| [Scaffolding](https://github.com/hashicorp/terraform-provider-scaffolding-framework) | Future implementation and release structure |
| [Best practices](https://developer.hashicorp.com/terraform/plugin/best-practices) | Resource/schema design |
| [Read](https://developer.hashicorp.com/terraform/plugin/framework/resources/read) | Refresh and missing-resource handling |
| [Import](https://developer.hashicorp.com/terraform/plugin/framework/resources/import) | Existing-resource adoption |
| [Plan modification](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification) | Replacement and unknown values |
| [Acceptance tests](https://developer.hashicorp.com/terraform/plugin/testing/acceptance-tests) | Lifecycle verification |
| [Actions](https://developer.hashicorp.com/terraform/plugin/framework/actions) | One-shot operations; current state update limitation |
| [Ephemeral resources](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources) | Nonpersistent temporary values |
| [Publishing](https://developer.hashicorp.com/terraform/registry/providers/publishing) | Naming, releases, checksums and signatures |
| [AWS instance](https://raw.githubusercontent.com/hashicorp/terraform-provider-aws/main/website/docs/r/instance.html.markdown) | Familiar instance fields and references |
| [AWS ingress rule](https://raw.githubusercontent.com/hashicorp/terraform-provider-aws/main/website/docs/r/vpc_security_group_ingress_rule.html.markdown) | Independently owned rules |
| [OpenStack provider](https://raw.githubusercontent.com/terraform-provider-openstack/terraform-provider-openstack/main/docs/index.md) | Standard protocol alternative; compatibility unverified |

These references inform original design decisions; their implementations have not been copied into this repository.
문서 기준일과 향후 live 검증일을 구분하고, 갱신 시 설계·검증 항목을 함께 검토합니다.
