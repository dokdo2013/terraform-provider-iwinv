# API 계약과 미확정 동작

[English](../en/api-contract.md) · [목차](../../README.md) · 리비전: 2

문서 조사일: 2026-09-18–19. 아래 표는 **문서에 기재된 계약**입니다.
인증된 조회와 통제된 변경 실측은 [검증 현황](contract-progress.md)에 별도로 기록합니다.
검증된 기능을 구현할 때는 문서 예시와의 차이를 반영합니다.
[기능 대장](../inventory/api.json)은 문서에 포함된 OpenAPI에서 사실을 추출한 것으로,
검증된 통합 OpenAPI 명세나 모든 서비스 조사 완료를 의미하지 않습니다.

## 확인된 계약과 검증 과제

| ID | 문서에서 확인한 내용 | 설계 영향 및 필요한 검증 |
| --- | --- | --- |
| C01 | 기본 API는 `https://api-kr.iwinv.kr`; Beta/Private Beta 표기 잔존 | 기존 계정 활성화·지원 존 확인 후 호환성 판단 |
| C02 | Timestamp와 path를 붙여 HMAC-SHA256; query 제외·끝 슬래시 제외; X-iwinv 헤더 3개 | 바이트 단위 서명, 경로 인코딩, 시도별 재서명, ±5분 시계 검증 |
| C03 | 분당 60회와 키별 허용 IP 제한 | quota 적용 단위, 429/Retry-After, CI 출발 IP 확인 |
| C04 | API 지원 존의 서버만 목록에 노출; 기본 page size 10 | 전체 페이지 조회; 빈 목록으로 계정 전체 서버 부재 판단 금지 |
| C05 | 생성 응답 202, 예시에 ID와 `building` 포함 | ID 확정 시점·대기 상태·실패 시 ID 보존 검증; 목록 enum에는 building 없음 |
| C06 | 상세 응답도 result 배열, 풍부한 정보는 fields 필요 | 결과 개수·비트마스크·누락/null·중첩 타입 검증; 단일 객체라고 가정 금지 |
| C07 | 상세 스키마에 zone/flavor/image/IP는 있지만 SSH/script 이력 없음 | import 복원 확인; 생성 응답의 ssh_key/user_script_id를 Read 지원 근거로 사용하지 않음 |
| C08 | 상세 응답에 default_account.password와 VNC 링크 포함 가능 | 최소 필드 요청, 로그 마스킹, 부수적으로 받은 비밀 값 저장 금지 |
| C09 | 이름/설명 길이 제한·URL 인코딩 지시, 조회 description은 null 가능 | 한글 NFC/NFD·바이트/글자·빈 값 삭제·단일/이중 인코딩 검증 |
| C10 | API는 한 번에 최대 10대 생성 | Terraform 리소스당 한 대만 생성하고 예상 밖 개수는 오류 처리 |
| C11 | Resize/rebuild/delete는 202 | 중단 시간·IP/디스크 영향·복구·quota·완료 상태와 과금 종료를 분리 검증 |
| C12 | Shutdown은 물리 전원 차단에 가까우며 정지 중에도 과금 | 안전한 OS 종료나 비용 종료로 설명하지 않음 |
| C13 | 검토한 생성 계약에서 멱등성 키 미확인 | 결과 불명확 시 재생성 금지; 요청 토큰·복구 방법 확인 |
| C14 | 블록 생성에 instance_id/type/size 필수, 응답에 연결 서버 ID | 독립 EBS 생성 모델을 가정하지 않고 분리·재연결·존·소유권 검증 |
| C15 | 블록 스토리지 GET과 보안 그룹 상세 GET에도 202 표기 | 상태 코드만으로 비동기 처리하지 않고 실제 응답별 의미 확인 |
| C16 | 보안 규칙 설명은 IN/OUT·TCP, 예시는 inbound/tcp, ID는 정수/문자 혼재 | 입력·정규값·ID·포트·CIDR·ICMP·중복 규칙 동작 확인 |
| C17 | 서버 생성의 security_group·attach_public_ip·block_storage.N.type는 deprecated | 대체 API를 검증한 뒤 사용; 폐기 입력을 안정 기능으로 약속하지 않음 |
| C18 | Script 신규 등록은 콘솔 전용, SSH 키도 문서/CLI에서는 조회만 확인 | 초기에는 조회·참조만; 생성/수정/삭제 미지원은 명시적인 API 공백으로 추적 |
| C19 | IaaS 외 5개 서비스의 26개 작업에 추출된 응답 스키마가 없음 | 실응답 확보 전 ID·대기·import·Data Source 스키마 확정 금지 |
| C20 | 일부 문서는 multipart 헤더와 application/json requestBody가 충돌 | 작업별 전송 형식 검증; 자동 생성 클라이언트를 그대로 신뢰하지 않음 |
| C21 | NAS/DBMS allowip PUT은 배열 입력, IP별 삭제는 미기재 | 추가/전체 교체·빈 집합·Read 확인 후 집합 소유권 결정 |
| C22 | Cache referrer PUT의 추출 스키마에 유효한 body 속성 없음 | 입력·초기화·조회 계약 확인 전 구현 보류 |
| C23 | 웹메일 계정은 생성/삭제만 있으며 별도 목록/상세 없음 | 부모 서비스 응답에서 조회 가능한지 확인; Read/import 없이 관리 리소스 출시 금지 |
| C24 | Object CLI는 목록/복사/이동/삭제 제공, presign은 15분 표기 | S3 인증·endpoint·호환 기능을 별도 검증; AWS S3 전체 호환 가정 금지 |
| C25 | 문자/알림톡은 control-plane과 다른 도메인·헤더 사용 | 서비스별 인증·오류·요청 제한 분리, 발송은 비멱등 Action 후보 |
| C26 | 알림톡 템플릿 수정은 검수 상태에 따라 제한 | 상태별 수정/교체·검수 대기와 전체 상태 전이 검증 |
| C27 | MCP OAuth와 mcp:tools 확인, 인증 후 도구 목록은 미확인 | 추후 tools/list와 기능 대장 비교, Provider 실행 의존성으로 쓰지 않음 |
| C28 | 필드 페이지는 OpenStack이라 쓰지만 인증은 자체 HMAC | 고객용 Keystone/표준 endpoint 제공 여부 확인 전 상호운용 주장 금지 |
| C29 | 웹 호스팅 삭제는 복구 불가, 동일 계정명은 삭제 후 24시간 재사용 불가로 문서화 | 단순 교체가 삭제 후 생성 실패를 유발할 수 있음. 새 계정명·데이터 이전·import 정책을 [호스팅 설계](webhosting-lifecycle.md)와 Core 테스트로 확정 |

출처: [요청](https://iwinv-common.readme.io/reference/api-request),
[응답](https://iwinv-common.readme.io/reference/api-response),
[키 관리](https://docs.iwinv.kr/developers/api/api-key-management/),
[인스턴스 생성](https://iwinv.readme.io/reference/postv1instances),
[상세 조회](https://iwinv.readme.io/reference/getv1instancesinstanceid),
[필드 목록](https://api-kr.iwinv.kr/fields/v1/instances),
[블록 생성](https://iwinv.readme.io/reference/postv1blockstorages),
[규칙 생성](https://iwinv.readme.io/reference/post_v1-security-groups-id-rules).
그 외 서비스와 CLI의 출처는 기능 대장 및 [출처 목록](../sources.md)에 연결되어 있습니다.

## 구현 확정 전에 필요한 증거

각 작업에 조사일, 메서드/경로, 인증 종류, 입력 인코딩·타입·제약·기본값, HTTP/업무 오류,
응답 ID, 완료 조건, 페이지, 일관성, 멱등성, quota 단위, 과금 영향, 지원 상품/존, 폐기 상태를 기록합니다.
Git에는 검토된 합성 또는 마스킹 fixture만 넣고 실제 계정 원본 응답은 저장하지 않습니다.
문서 갱신 시 변경·추가·삭제된 계약을 비교해 검토합니다.

`response_schema_present`는 문서에 스키마가 있다는 표식일 뿐입니다. null 전용 속성이나 타입 불일치도 있어
실제 응답으로 nullable/optional을 확정해야 합니다. 알 수 없는 성공 코드를 임의로 성공 처리하지 않습니다.

## 공급사에 확인할 질문

1. API 정식 제공 여부와 기존 상품·존·계정 지원 범위는 무엇인가?
2. 버전/폐기 보장, 통합 OpenAPI와 테스트 환경이 있는가?
3. 생성 멱등성 토큰·요청 ID·작업 상태 조회·응답 유실 복구를 지원하는가?
4. SSH 키·스크립트 관리와 Read, 웹메일 계정 목록/상세를 지원하는가?
5. 작업별 실제 인코딩·허용 목록 교체 의미·분당 60회 적용 단위는 무엇인가?
6. 독립 블록 스토리지가 가능한가? 서버 삭제·디스크 보존·과금은 어떻게 연결되는가?
7. 공개 API 문서에 없지만 공식 CLI/MCP로 지원하는 작업이 있는가?
8. 고객이 접근 가능한 표준 OpenStack endpoint가 있는가?

질문만 준비했으며 공급사에 발송하지 않았습니다. 답변은 Cxx 항목·기능 대장·스키마 결정·검증 조건에 함께 반영합니다.

## NAS 입력 타입 추가 확인

공식 NAS 생성 문서의 `allowip` OpenAPI 타입은 `array(string)`이지만 설명은 IP를 key로,
`RW`/`RO`를 value로 가지는 object 예시입니다. C20/C21의 추가 문서 불일치입니다.
요청 헤더의 multipart 표기와 JSON requestBody 차이도 함께 남아 있으므로
후속 실측에서 JSON object 생성과 map 전체 교체를 확인했고, 빈 object는 HTTP 422로 거부됐습니다.
[실측 근거](contract-progress.md)를 반영하되 나머지 서비스로 일반화하지 않습니다.
출처: [NAS 생성](https://iwinv-api-nas.readme.io/reference/%EA%B3%B5%EC%9C%A0-%EC%8A%A4%ED%86%A0%EB%A6%AC%EC%A7%80-%EC%83%9D%EC%84%B1).

## 추가 서비스 실측 보완

[한정 실험](contract-progress.md)으로 C19–C23을 보완했습니다. 캐시는 중첩 `pw.FTP`가 필요하고 생성 시 레퍼러를 무시합니다.
JSON 레퍼러 전체 교체는 성공하지만 빈 배열 clear는 거부됩니다. DBMS 허용 IP도 전체 교체되며 빈 배열을 거부합니다.
캐시·웹메일 수정은 작업 중에도 404 / `NOT_FOUND`를 반환하므로 이를 보편적인 부재 신호로 쓰면 안 됩니다.
웹메일은 생성 성공 직후 목록에서 일시 누락될 수 있고 Read에 이름과 하위 계정이 없습니다.
계정 생성은 비밀번호를 그대로 반환하므로 진단이나 부수적인 state에 포함하지 않습니다.
이 서비스별 관찰은 import·정리 일관성·전체 수명주기 계약의 완료를 의미하지 않습니다.

웹메일은 추가 콘솔 대조에서 API의 성공한 빈 목록과 실제 서비스 잔존이 동시에 관찰됐습니다.
C23의 Read 공백에 부모 서비스의 작업 중 누락도 포함합니다. 이 API의 빈 목록만으로 Terraform state를 제거하면 안 됩니다.
[콘솔 대조 근거](contract-progress.md)를 먼저 해결해야 합니다.
