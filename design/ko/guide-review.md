# 기능별 가이드 동작 검토

[English](../en/guide-review.md) · [문서 정책](documentation.md)

2026-09-19 등록된 Data Source 18개와 리소스 7개의 한국어·영어 기능 가이드를 `fb654c3406e697b0b7d0a0289ff3e3cbbbcec9a8` 구현과 대조했습니다. 검토 범위는 아래 표의 지원 입력, 식별자, 전체 목록 소유권, 수정·교체, import, 빈 결과와 실패 복구입니다. 범위를 정한 수동 검토이며 모든 문장의 증명이나 새로운 실환경 acceptance 실행은 아닙니다. 이 검토의 수정은 안내와 오류 메시지 하나를 바꾸며 검증기·스키마·원격 동작은 바꾸지 않습니다.

## 수정 사항

- 보안 그룹 단건 가이드에 목록 엔드포인트의 페이지 처리 설명이 들어 있었습니다. 정확한 ID의 상세 요청 한 번과 결과가 없을 때의 진단으로 고쳤습니다. 목록 가이드에서는 받지 않는 ID 입력 설명을 제거했습니다. 양언어 모두 적절한 다른 조회 가이드로 연결합니다.
- 웹호스팅 비밀번호 안내와 오류 메시지에 공백 금지를 명시했습니다. `validHostingPassword`와 기존 `Has space1!` 거절 테스트에 맞춘 수정이며 7–20자, 문자 종류 두 가지 이상이라는 지원 범위는 그대로입니다.
- 웹호스팅의 로컬 `password_wo_version`을 제한 없는 number 대신 양의 정수(`int64`)로 설명합니다. 계정 전체 교체를 유발하며 비밀번호만 제자리에서 바꾸는 기능은 아닙니다.

## 대조한 항목과 구현 근거

아래 이름에는 모두 `iwinv_` 접두사가 붙습니다. 각 항목은 `docs/`와 대응하는 `docs/ko/` 기능 페이지 양쪽을 포함합니다. 링크는 대조한 구현이며 공급사의 모든 상품 변형에 실환경 근거가 있다는 뜻은 아닙니다.

| 기능 페이지 | 대조 항목 | 구현 |
| --- | --- | --- |
| `availability_zones` | 정확한 ID·상태, 순서가 일치하는 정렬 출력, count·중복 검증, 조회 가능 여부와 생성 권한의 구분. | [존 어댑터](../../internal/services/compute/zones.go), [출력 변환](../../internal/provider/availability_zones_data_source.go) |
| `images`, `image` | 10행씩 짧은 페이지까지 조회와 정확한 상세 조회의 차이, ID 형식, 최신·비공개 이미지 추정 금지. | [카탈로그 어댑터](../../internal/services/compute/catalogs.go), [Data Source](../../internal/provider/catalog_data_source.go) |
| `instance_types`, `instance_type` | total 기반 순회, 점을 포함한 flavor ID, 일치하는 상세 한 건, CPU·가격·존 필드 추정 금지. | [카탈로그 어댑터](../../internal/services/compute/catalogs.go), [Data Source](../../internal/provider/catalog_data_source.go) |
| `ssh_keys`, `ssh_key` | 전체 목록 검증 후 정확한 ID 선택, 빈 이름·중복 ID 처리, 키 내용 제외. | [SSH 어댑터](../../internal/services/compute/ssh_keys.go), [Data Source](../../internal/provider/ssh_key_data_source.go) |
| `webhosting_products`, `webhosting_servers` | SHARE/SINGLE 필터, 상품·서버 ID 정렬, PHP 라벨, 정확한 정수 서버 ID와 조회 불가 생성 이력. | [카탈로그 어댑터](../../internal/services/hosted/webhosting_catalog.go), [Data Source](../../internal/provider/webhosting_catalog_data_source.go) |
| `db_instance_products` | 엔진·등급 필터, 빈 ID와 버전 간 반복 ID, 생성 버전 선택자의 부재. | [DBMS 어댑터](../../internal/services/hosted/dbms.go), [출력 변환](../../internal/provider/db_instance_products_data_source.go) |
| `content_cache_products` | null·빈 ID 구분, 전체 행과 필터 일치, 카탈로그 종류와 격리 보장의 구분. | [캐시 어댑터](../../internal/services/hosted/cache.go), [출력 변환](../../internal/provider/content_cache_products_data_source.go) |
| `shared_storage_products` | 입력 없음, nullable 버전, 정확한 용량 범위, 빈 ID 행, 증설 대신 교체. | [NAS 어댑터](../../internal/services/hosted/nas.go), [출력 변환](../../internal/provider/shared_storage_products_data_source.go) |
| `webmail_products` | 빈 ID 상품 행, 안정 정렬, 서비스·메일 계정 수명주기 미지원. | [웹메일 어댑터](../../internal/services/hosted/webmail_catalog.go), [출력 변환](../../internal/provider/webmail_products_data_source.go) |
| `block_storage_types` | 선택적 정확한 필터, null·빈 존 목록, int64 용량, 엄격한 응답 검증, 볼륨 수명주기 미지원. | [스토리지 어댑터](../../internal/services/storage/types.go), [Data Source](../../internal/provider/block_storage_types_data_source.go) |
| `current_bill`, `bills` | 예상 청구 한 건과 페이지별 요약 목록의 차이, 민감 출력, 원문 금액, null·unknown 필터, 첫 페이지 EMPTY_SET의 한정 처리. | [청구 어댑터](../../internal/services/billing/billing.go), [Data Source](../../internal/provider/billing_data_source.go) |
| `security_groups`, `security_group` Data Source | 목록·상세 경로 구분, 정확한 ID, 설명 한 번 디코딩, 규칙·연결을 소유하지 않음. | [그룹 어댑터](../../internal/services/network/groups.go), [Data Source](../../internal/provider/security_group_data_source.go) |
| `security_group` 리소스 | 기본값·길이, 제자리 속성 수정, import 식별자, 쓰기 실패 state 유지, 검증된 빈 상세 응답 계약. | [리소스](../../internal/provider/security_group_resource.go), [어댑터](../../internal/services/network/groups.go) |
| `security_group_ingress_rule`, `security_group_egress_rule` | 복합 ID, 프로토콜·포트·CIDR, 고정 방향, 설명 초기화·부모 변경 시 교체. | [리소스](../../internal/provider/security_group_rule_resource.go), [어댑터](../../internal/services/network/rules.go) |
| `webhosting` | 초기 write-only 비밀번호, 계정 전체 교체, 새 계정명, 과거 서버 입력, 도메인과 생성 실패 복구. | [리소스](../../internal/provider/webhosting_resource.go), [어댑터](../../internal/services/hosted/webhosting.go) |
| `db_instance` | 비어 있지 않은 전체 IPv4 집합, 계정 이력, 허용 목록 제자리 수정과 파괴적 교체의 차이, 엔진 버전 선택자 부재. | [리소스](../../internal/provider/db_instance_resource.go), [어댑터](../../internal/services/hosted/dbms.go) |
| `content_cache` | 전체 리퍼러 소유, 빈 목록·unknown의 교체, write-only 비밀번호 트리거, 한정된 busy 재시도, 식별자 유지. | [리소스](../../internal/provider/content_cache_resource.go), [어댑터](../../internal/services/hosted/cache.go) |
| `shared_storage` | 비어 있지 않은 IPv4→RO/RW 맵, 공유 이름 이력, 용량 교체, 수정 전 부모 대조, 자동 쓰기 재시도 없음. | [리소스](../../internal/provider/shared_storage_resource.go), [어댑터](../../internal/services/hosted/nas.go) |

## Provider 소개와 공통 가이드

후속 검토는 `5fdf4f856bd2988c169e4d29722049a3b14407e7` 구현과 남은 페이지 쌍 세 개를 대조하여 현재 언어별 28페이지의 검토 목록을 채웠습니다.

| 페이지 쌍 | 대조 항목 | 근거 |
| --- | --- | --- |
| Provider 소개 | 환경변수·설정 우선순위, alias별 독립 클라이언트, 고정 endpoint, CLI profile 로그인 미사용, 개발 override, 민감 값, 지원 범위와 요청 시간. | [Provider 설정](../../internal/provider/provider.go), [클라이언트](../../internal/client/client.go), [설정 테스트](../../internal/provider/provider_test.go), [개발 절차](development.md) |
| 보안 그룹 규칙 가이드 | 규칙 하나의 소유권, 방향 복원, 설명·부모 교체, 전체 규칙 목록 검증, 부모 우선 부재 판정, 쓰기 실패 state, 실제 테스트 opt-in과 정리 대장 조건. | [규칙 어댑터](../../internal/services/network/rules.go), [리소스](../../internal/provider/security_group_rule_resource.go), [비공개 대장 실환경 실행기](../../internal/provider/security_group_rule_live_test.go) |
| 생성 스키마 참조 | 필수·선택·계산과 조건부 필수의 차이, 상속 민감 표시, 쓰기 전용 범위, number 타입과 구현 정수 범위, 스키마 버전과 릴리스 버전 구분. | [참조 생성기](../../scripts/check_intro_docs.py)와 같은 스크립트의 실행 스키마 비교 |

소개에 30초 HTTP 요청 제한과 리소스 전체 작업 시간, 설정별 1초 요청 간격과 계정 quota의 차이를 추가했습니다. 서명 없는 설치와 일회용 키 서명 검증을 모두 연결하되 Registry 신뢰 검증으로 설명하지 않습니다. 키의 공백·줄바꿈 조건도 클라이언트와 맞췄습니다. 공통 규칙 가이드에는 [공식 lifecycle 문서](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle#prevent_destroy)에 따라 `prevent_destroy`가 설정에만 적용됨을 명시합니다. 새 스키마 표시·timeout 설정·재시도·클라우드 작업은 추가하지 않습니다.

## 검증의 범위와 남은 작업

기존 어댑터·Terraform Core 테스트가 동작의 실행 근거이며 수동 검토가 이를 대신하지 않습니다. 특히 [웹호스팅 입력 테스트](../../internal/services/hosted/webhosting_test.go)는 공백 포함 비밀번호를 요청 전에 거절하고 두 비밀번호가 진단에 포함되지 않는지 이미 검사합니다. [보안 그룹 Data Source 테스트](../../internal/provider/security_group_data_source_test.go)는 목록·상세, unknown 입력, 잘못된/없는 ID와 API 오류를 구분합니다. 이번 수정에는 새 클라우드 리소스가 필요하지 않았습니다.

T052는 `in_progress`로 유지합니다. 현재 Provider 소개·공통 가이드·기능 페이지는 위 범위로 검토했으며 실제 게시된 Registry 이동, 버전별 목적지와 양언어 사이트는 출시 작업입니다. 앞으로 동작이나 설명을 변경하면 다시 검토해야 합니다. 바뀐 기능 페이지 6개는 공식 Registry 본문 미리보기에서 다시 확인했습니다. 제목과 수정 문구가 표시되고 머리말은 숨겨졌으며 표 6개가 모두 렌더링됐습니다. 후속 소개·규칙 가이드 수정도 네 페이지 모두 미리봤습니다. 제목·추가 문구가 보이고 머리말은 숨겨졌으며 예상 표 4개가 렌더링됐습니다. 바뀐 열 [페이지 기록](../inventory/doc-preview.json)에 관측한 내용 해시와 재검증 날짜를 기록하고 변경 없는 페이지는 이전 근거를 유지합니다. T080은 계속 구조·스키마·예제 검사만 다룹니다.

제어 API acceptance가 패킷 필터링·DB/파일 접속·가격·과금 종료까지 입증하지는 않습니다. 상품별 실환경 한계, 미해결 웹메일 정리와 미동의 OAuth 등록 정리도 그대로 남습니다. Compute 어댑터는 아직 미등록 후보이며 이번 가이드 수정으로 사용 가능한 인스턴스 리소스나 Registry 릴리스가 추가되지 않습니다.
