# P1 API 계약 검증 진행 현황

[English](../en/contract-progress.md) · [검증 계획](verification.md)

관측일: 2026-09-19 UTC. [이슈 #1](https://github.com/dokdo2013/terraform-provider-iwinv/issues/1)의 중간 구현 기록이며,
정식 릴리스 또는 P1 완료를 의미하지 않습니다. 개발용 Provider의 존 Data Source는 [실행 가이드](development.md)를 참고하세요. 현재 클라이언트는 단일 GET 요청을 지원합니다.

## 구현하고 검증한 내용

- 호출마다 새 Timestamp를 사용하는 control-plane HMAC-SHA256. query는 서명에서 제외하고 정규 ASCII 경로만 허용합니다.
- TLS 검증, 모든 redirect 거부, HTTP 30초 제한과 응답 크기 2 MiB 제한.
- 클라이언트당 1초 간격 요청 허용. 별도 프로세스의 합산 quota까지 보장하지 않습니다.
- HTTP 상태와 업무 응답을 각각 검사하고 알 수 없는 성공 표현은 오류로 처리합니다.
- 오류에 응답 본문·서버 메시지·URL·인증정보를 포함하지 않습니다.
- 서비스 decoder가 검증할 때까지 누락/null/빈 값을 Raw JSON으로 구분해 보존합니다.
- 합성 테스트에서 서명, 한글 query 인코딩, 오류, redirect, 취소, 클라이언트 격리,
  병렬 호출, 응답 크기 제한, 값을 노출하지 않는 진단 출력을 확인합니다.

아직 쓰기·재시도·페이지 처리·리소스 부재 판정은 구현하지 않았습니다.
202를 포함한 HTTP 상태를 보존해 서비스별로 의미를 판단하게 합니다.
진단 도구는 HTTP 200과 존 객체 배열만 허용합니다.

## 인증된 API 관측

IP 제한이 있는 임시 키로 읽기 전용 요청을 실행했습니다. 인증정보·계정 응답 원문·
실제 리소스 ID·상태 파일은 저장소에 포함하지 않습니다.

| 작업 | 관측 결과 | 남은 검증 |
| --- | --- | --- |
| 존 | HTTP 200; `zone_id`, `zone_name`, `status`는 문자열, count는 숫자 | 콘솔 기준 계정/존 전체 범위 대조 |
| 상품 | 10/10/10/3개씩 4페이지, 중복 없는 ID 33개, 숫자 `total` | 목록이 조회 중 변경되는 경우 |
| 이미지 | 10/10/10/10/0개씩 5페이지, 중복 없는 ID 40개, `total` 없음 | 상세 매핑과 정확한 필터 조회 |
| 마지막 이후 페이지 | 상품/이미지 모두 HTTP 200, 빈 배열, count 0 | 나머지 서비스는 별도 검증 필요 |
| 인스턴스 목록 | 테스트한 계정/API 범위에서 HTTP 200과 빈 결과 | 기존 서버의 상세/import 근거 없음 |
| SSH 키 목록 | HTTP 200, 배열, 문자열 키 ID, 숫자 페이지 메타데이터 | 문서에서 키 변경 API는 확인되지 않음 |
| 오래된 Timestamp | 현재보다 600초 이전으로 서명한 존 요청은 HTTP 401 | 정확한 허용 경계와 전용 오류 분류 |

T001, T002, T004, T006, T009의 일부 근거를 확보했습니다. T003은 아래 2026-09-23의
동일 키·경로 인증 재대조로 통과했습니다.
T011은 모의 클라이언트 격리와 실제 바이너리의 정상 키/합성 오류 키 alias 분리로 검증했습니다. redirect/오류 비밀정보 차단 테스트는
현재 모의 테스트 전용 항목인 T012의 범위를 충족합니다. 리소스 수명주기 acceptance의 근거는 아닙니다.

## 로컬 실행

```sh
go test -race -cover ./...
go vet ./...
python3 scripts/check_docs.py
```

선택적으로 인증된 진단을 실행하려면 로컬 비밀정보 관리자/환경변수로
`IWINV_ACCESS_KEY`, `IWINV_SECRET_KEY`를 주입한 뒤 실행합니다.

```sh
go run ./cmd/contract-probe
```

진단 도구는 `/v1/zones`에 GET 요청을 한 번 실행합니다. 허용된 필드의 타입만 출력하며
원격 ID·이름·비밀 값은 출력하지 않습니다. CLI 프로필 조회, endpoint 변경, state 저장,
리소스 생성 또는 acceptance 합격 선언은 하지 않습니다. 오류가 있으면 비정상 종료합니다.
키를 셸 기록에 남기지 말고 테스트 세션 종료 후 임시 키를 폐기하세요.

실제 변경 테스트는 해당 실행에서 생성한 ID만 기록해 순차 처리하고 정리 여부를 검증합니다.
빈 계정 목록은 임의 객체를 삭제할 수 있다는 근거가 아닙니다.

## 호환성 결정 (ADR-0001, 잠정)

모듈 최소 Go 버전은 1.25.8, CI는 1.25.8과 1.26.1로 구성합니다. 클라이언트 자체는 표준 라이브러리를 사용하며 Provider 의존성은 go.mod에 고정합니다.
로컬 계약 테스트는 Go 1.26.1, macOS arm64에서 실행했습니다. Terraform 1.14.2에서
합성 프로토콜 테스트와 존 Data Source의 실환경 읽기 acceptance/무변경 plan을 통과했습니다.

확인한 [공식 scaffolding 의존성 기준](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/go.mod)에
맞춰 Framework 1.19.0과 plugin-testing 1.16.0을 고정해 컴파일·테스트했습니다.
초기 Provider의 호환 목표는 Terraform >=1.14로 정하고 출시 전 최소 버전을 별도 검증합니다.
[Ephemeral Resource](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources)는 >=1.10,
[write-only 인자](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments)는 >=1.11이 필요하지만
개별 기능의 하한이 이 프로젝트의 목표 하한을 낮추지는 않습니다.
Action은 공개 전에 별도 acceptance와 state 정합성 검증을 통과해야 합니다.
T013의 Provider/프로토콜 조합은 Go 1.25.8·1.26.1 및 Terraform 1.14.0·1.14.2 CI에서 통과했습니다.
[Action](https://developer.hashicorp.com/terraform/plugin/framework/actions/testing)은 Terraform >=1.14가 필요하며 실제 Action 지원을 선언하는 것은 아닙니다.

출처: [iwinv 서명](https://iwinv-common.readme.io/reference/api-request),
[응답](https://iwinv-common.readme.io/reference/api-response),
[존](https://iwinv.readme.io/reference/getv1zones),
[상품](https://iwinv.readme.io/reference/getv1flavors),
[이미지](https://iwinv.readme.io/reference/getv1images).

## 추가 기능 조사와 첫 변경 실험

공식 [CLI v0.2.2 명령·옵션 대장](../inventory/cli.json)은 루트와 completion을 포함해 47개 도움말 범위를 기록합니다.
CLI에 계정을 로그인하지 않고 조사했습니다. 설치 스크립트는 `/usr/bin`을 대상으로 하므로
시스템 설치 대신 임시 바이너리로 도움말만 확인했습니다.

[추가 서비스 작업 대장](../inventory/service-operations.json)에 NAS·Cache·Swift·S3 기능을 기록했습니다.
Object Storage는 두 프로토콜과 별도 키를 문서화하며 공개 endpoint는 `kr.object.iwinv.kr`입니다.
[인증](https://help.iwinv.kr/manual/712), [호환 범위](https://help.iwinv.kr/manual/738)를 참고하세요.
[이미지 표의 전사 대장](../inventory/object-compatibility.json)에 S3 22개·Swift 22개 기능의 공급사 표기를 기록했습니다. 실환경 검증은 별개입니다. CLI의 `obs://`만으로 S3 전체 호환을 판단하지 않습니다.
`x-amz-security-token` 요청 및 version/delete-marker 응답 헤더는 미지원으로 표기되어 있어 STS와 버전 관리 의미를 별도 확인해야 합니다.
Swift Temporary URL의 컨테이너 키와 `X-History-Location`은 지원하지 않는다고 명시되어 있습니다.
S3 정책 문법에는 AWS 형식 ARN이 실제로 쓰일 수 있지만 iwinv control-plane IAM/ARN 지원을 뜻하지 않습니다.
Swift와 S3 리소스가 같은 버킷/객체를 동시에 소유하게 만들지 않습니다.

[NAS 매뉴얼](https://help.iwinv.kr/manual/763)의 이름/태그 변경은 요약에서 PUT, 상세 표에서 DELETE로 나옵니다.
중요한 불일치이므로 메서드를 미확정으로 남기고 이름 변경을 위해 DELETE를 추측해 호출하지 않습니다.
[Cache](https://help.iwinv.kr/manual/938)도 tenant별 토큰 인증을 사용하며 control-plane HMAC과 별개입니다.
일반 SDK 소개 페이지에서는 다운로드 가능한 SDK 패키지를 확인하지 못했습니다.
인증 후 MCP 조사는 OAuth가 필요하며 control-plane 키를 bearer token으로 대체하지 않습니다.
이는 T014의 부분 결과이며 전체 기능 조사 완료를 의미하지 않습니다.

읽기 전용 검사로 나머지 5개 서비스의 상품 목록에 접근했습니다. 테스트 계정의 서비스 목록은 비어 있었습니다.
상품 배열에는 표준 `count`/페이지 정보가 없었고 Cache에는 null인 `product_id`도 있었습니다.
최초 호스팅 서버 목록 요청은 `product_id` 없이 보내 404였고 User Script 목록도 404였습니다. 호스팅은 필수 상품 query를 넣은 후 성공했습니다(아래 참조). Script 경로는 미해결입니다. 성공한 빈 목록과 404를 무조건 같은 값으로 처리하면 안 됩니다.
블록 스토리지 목록은 과거 문서의 202와 달리 HTTP 200을 반환했습니다.

T005/T015 검증을 위해 작은 Linux 상품, 호환 존/이미지, 기존 SSH 키 참조 하나로 범위가 정해진 multipart 생성 요청을 한 번 보냈습니다.
HTTP 500과 `DEV_CHECK_RETURN`이 반환되었고 인스턴스 ID는 없었습니다. result는 성공 문서의 배열 대신 오류 객체였습니다.
[공식 오류 표](https://api-kr.iwinv.kr/error)는 이를 인증 오류가 아닌 서버 반환값 문제로 분류합니다.
자동 재시도나 이름 기반 채택은 하지 않았습니다. 직후와 지연 후 인스턴스 목록은 비어 있었습니다.
원인·백엔드 처리 결과·과금 상태는 미검증이므로 compute CRUD 공개는 보류합니다.
이 API 계약 실험의 T015는 실패이며 Terraform 인스턴스 리소스 acceptance를 수행했다고 하지 않습니다.

[CI 실행 증거](../inventory/evidence.json): 소스 commit과 실제 성공한 실행 링크를 기록합니다.

## 이미지·상품 조회 구현 증거 (2026-09-19)

`iwinv_images`, `iwinv_image`, `iwinv_instance_types`, `iwinv_instance_type`를 추가했습니다.
조회 전용 Terraform acceptance로 전체 목록, 공개 이미지 상세, 상품 상세와 후속 무변경 plan을 검증했습니다.
페이지 크기 10에서 이미지는 40개와 마지막 빈 페이지, 상품은 total 33과 마지막 3개 페이지를 관찰했습니다.
이는 해당 실행의 카탈로그 수치이며 코드나 테스트에 고정하지 않습니다.
없는 합성 ID에 대해 이미지 상세는 HTTP 400 / `ID_INVALID`, 상품 상세는 HTTP 200 / 빈 배열이었습니다.
두 경우 모두 조회 실패로 처리합니다. 단일 조회의 정확한 ID 일치와 결과 수 1도 검사합니다.

[이미지 상세](https://iwinv.readme.io/reference/getv1imagesimageid)는 공개/비공개 응답 차이를 명시합니다.
비공개 이미지와 전체 상세 필드는 검증되지 않았으므로 제한된 출력만 제공합니다.
[상품 상세](https://iwinv.readme.io/reference/getv1flavorsflavorid)의 ID는 점을 포함할 수 있으며 그대로 사용합니다.
모의 테스트는 중복·잘못된 metadata·변하는 total·도중 실패·페이지 상한·취소를 검사합니다.
목록 조회의 정합성 개선이며, 생성 실패나 다른 서비스의 페이지 계약을 해결했다는 의미는 아닙니다.

## 보안 그룹·규칙 API 실측 (2026-09-19)

서버 생성 실패와 독립적으로, 연결되지 않은 테스트 보안 그룹 3개를 순차적으로 생성해 계약을 확인했습니다.
기존 그룹·서버에는 변경하지 않았습니다. 매번 생성 응답의 정확한 문자열 ID를 비공개 정리 대장에 저장했습니다.
이는 API 직접 실측이며 Terraform 보안 그룹 Resource 구현·acceptance·import 완료를 의미하지 않습니다.

| 작업/입력 | 실측 결과 | 구현 시 의미 |
| --- | --- | --- |
| 그룹 생성·상세·수정 | 모두 HTTP 200, `firewall_id` 문자열 | 문서의 202/정수 생성 ID 예시를 그대로 모델링하지 않음 |
| 그룹 이름·설명·ICMP 수정 | 지정한 값과 재조회 값 일치, `N`→`Y` 유지 | 명시적 쓰기 후 Read 검증 가능 |
| 규칙 생성·수정 | HTTP 200, `rule_id` 정수 | Terraform 문자열 ID와 API 숫자 ID 변환을 명시적으로 설계 |
| `IN`/`TCP`, 단일 포트, /32 | 생성·재조회 성공 | 검증된 대문자 표기 사용 |
| 문서 예시의 `inbound`/`tcp` 조합 | HTTP 400 / `CHECK_PARAM_ENUM` | 두 값 중 어느 값만의 원인인지는 이 조합 테스트로 구분하지 않음 |
| `OUT`/`UDP`, `10000-10002`, /24 | 생성 성공, 입력값 보존 | 포트 범위·출력 방향 검증의 부분 근거 |
| `192.0.2.1/24` | 성공 응답에서 호스트 비트가 있는 표기 그대로 반환 | 임의로 네트워크 주소로 바꾸면 state 불일치 가능; 패킷 동작은 미검증 |
| 규칙 포트·설명만 PUT | 생략한 방향·프로토콜·IP·이름 유지 | 관찰한 부분 수정 동작 기록 |
| 규칙 DELETE 후 부모 규칙 목록 | HTTP 200 / 빈 배열 / count 0 | 개별 삭제의 부재 확인 |
| 그룹 DELETE 후 상세 | HTTP 200 / 빈 배열 / count 0 | 이 endpoint에서 관찰한 삭제 판정 후보; 모든 200을 부재로 취급하지 않음 |
| 규칙이 있는 부모 삭제 후 규칙 목록 | HTTP 400 / `CHECK_PARAM` | 이 오류만으로 규칙 부재를 판단하지 않고 부모의 정확한 부재를 먼저 확인 |

모든 테스트 그룹은 삭제 응답 이후 상세의 빈 결과와 전체 그룹 목록에서 부재를 확인했습니다.
한 규칙은 개별 삭제 후 부재를 검증했습니다. 나머지 두 규칙은 부모 삭제 후 별도 조회가 불가능하므로
서버 내부의 물리적 cascade 삭제까지 입증하지는 않습니다. 테스트 그룹은 어떤 인스턴스에도 연결하지 않았습니다.
중복 규칙, 두 값 각각의 소문자 허용 여부, IPv6, ICMP 패킷 동작, import, 외부 변경, 연결 cardinality는 남아 있습니다.
C15/C16과 T031/T032의 부분 근거이며 완료로 표시하지 않습니다.

출처: [그룹 생성](https://iwinv.readme.io/reference/post_v1-security-groups),
[그룹 상세](https://iwinv.readme.io/reference/get_v1-security-groups-id),
[규칙 생성](https://iwinv.readme.io/reference/post_v1-security-groups-id-rules),
[그룹 삭제](https://iwinv.readme.io/reference/delete_v1-security-groups-id).

## 생성 제한 원인과 쓰기 클라이언트 (2026-09-19)

존·상품 제공 여부를 다시 확인하고 이전 생성 이름이 지연 목록에 없음을 확인한 뒤,
설명·SSH 키·스크립트를 생략한 별도 최소 multipart 진단 요청 한 번을 수행했습니다.
HTTP 500 / `DEV_CHECK_RETURN`이 다시 발생했고, 내부 결과에는 숫자 code 12와
“리소스 사용이 제한된 ZONE입니다.”라는 메시지가 있었습니다. 원문에 포함된 실행 코드는 실행하거나 Provider 진단에 노출하지 않습니다.
이 결과는 생성 제한을 보여주지만 계정/존 정책 중 어떤 설정이 원인인지는 공급사 확인이 필요합니다.
생성 ID는 없었고 직후 API 목록은 비어 있었습니다. C05/T015는 해결되지 않았습니다.

공통 Go 클라이언트에 JSON·multipart 쓰기와 DELETE를 추가했습니다.
서명, 빈 값/생략 구분, 한국어·특수문자 전송, payload 제한, 리다이렉트 차단, HTTP 오류·끊어진 연결에서의 비재전송을 합성 테스트로 검사합니다.
실제 Go 클라이언트로 별도 테스트 보안 그룹의 JSON 생성·수정·삭제와 삭제 후 빈 상세 결과를 검증했습니다.
이 ASCII 전용 실험에서는 빈 문자열 PUT이 HTTP 200이어도 이전 설명을 유지했습니다. 처음에는 clear 성공을 기대한 테스트가 실패했습니다.
아래 이스케이프 문자열 추가 실측에서 이를 보편적인 무동작으로 볼 수 없음이 드러났고, 현재 어댑터는 빈 값 수정을 거부합니다.
미래 Resource는 이를 clear 성공으로 state에 기록해서는 안 됩니다. 모든 Go 테스트 그룹은 삭제 후 부재를 확인했습니다.

## NAS control-plane 부분 수명주기 (2026-09-19)

사용 가능한 최소 디스크 상품을 선택해 임시 NAS 하나를 생성하고 허용 IP를 변경한 뒤 삭제했습니다.
문서에 `array(string)`으로 적힌 `allowip`는 `{ "192.0.2.1": "RO" }` 형태의 JSON object로 생성에 성공했습니다.
아래 주소는 문서용 합성 주소이며 실제 접근 주소나 계정 식별자가 아닙니다.

| 항목 | 실측 | 남은 검증/설계 영향 |
| --- | --- | --- |
| POST JSON | HTTP 200, result는 단일 object, `service_idx` 정수 | IaaS의 result 배열 decoder를 재사용하지 않음 |
| GET 목록 | result 배열, count/page 없음, 동일 ID와 설정 확인 | 전체 목록·외부 추가/삭제 정합성은 추가 검증 |
| 상태 | 생성·즉시 Read에서 `pending` | HTTP 200은 실제 스토리지 준비 완료가 아님; ready/mount/과금은 미검증 |
| `allowip` | Read에서도 IP→RW/RO object | map 소유권 설계 근거 |
| PUT `{ "192.0.2.2": "RW" }` | 기존 IP 제거, 새 IP만 남음 | 단일 Resource가 전체 허용 집합을 소유해야 함 |
| PUT 빈 object | HTTP 422, `message`/`errors.allowip`, 기존 값 유지 | 빈 집합 clear 미지원 관측; 일반 성공 envelope를 가정하지 않음 |
| Read 필드 | `spec.disk` 정수, `stop_date` null, `mount_info` 문자열 | 실제 domain/mount 값은 공개 fixture·로그에 포함하지 않음 |
| DELETE | HTTP 200, result 문자열; 후속 목록에서 정확한 ID 부재 | 이 테스트 NAS의 삭제 확인 |

C19–C21과 T037/T039의 부분 근거입니다. 프로비저닝 완료를 기다리거나 데이터를 기록·마운트하지 않았고,
서비스 API 토큰 인증·파일 작업·Terraform lifecycle/import를 검증하지 않았습니다. NAS Resource는 아직 구현하지 않았습니다.
[공식 NAS 생성 문서](https://iwinv-api-nas.readme.io/reference/%EA%B3%B5%EC%9C%A0-%EC%8A%A4%ED%86%A0%EB%A6%AC%EC%A7%80-%EC%83%9D%EC%84%B1).

## 호스팅·캐시·DBMS 계약 실측 (2026-09-19)

한정된 API 실험으로 C19–C22와 T005/T037/T039 근거를 보강했습니다. Terraform 리소스 등록이나 T038 수명주기/import acceptance 완료를 뜻하지 않습니다.
생성한 호스팅·캐시·DBMS는 모두 정확한 생성 ID를 비공개 기록한 뒤 삭제했고 서비스 목록의 부재를 확인했습니다.
기존 리소스·운영 DB·메시지 수신자를 변경하지 않았습니다.

| 서비스 | 실측 결과 | 설계 영향 / 미해결 사항 |
| --- | --- | --- |
| 호스팅 서버 상품 조회 | `GET /v1/webhosting/servers?product_id=…` 성공; 정수 `idx`, PHP/DB 정보 반환 | 앞선 무필터 404는 경로 미지원의 증거가 아님 |
| 호스팅 생성/조회 | JSON 생성 성공; object `result`와 정수 `service_idx`; `pending` → `active`; 한글과 `& + %` 설명 보존 | Read에 `server_idx`/PHP 선택 이력이 없어 해당 입력을 import로 복원할 수 없음 |
| 캐시 비밀번호 | 문서의 `ftppw`는 422로 `pw`/`pw.FTP` 요구; 문자열 `pw`도 실패; JSON `pw: {"FTP": "…"}` 성공 | 검증한 중첩 입력 사용; 실제 생성 비밀번호는 공개하지 않음 |
| 캐시 생성/조회 | 생성에 `product_id` 없음, Read에는 있음; `pending` → `active`; 생성 시 `allow_referer` 무시 | 전체 필드 검증 전에 생성 ID를 보존하고 생성 후 소유 레퍼러를 별도 적용 |
| 캐시 레퍼러 PUT | JSON 배열로 전체 교체; 1개/2개 항목 모두 그대로 조회 | 전체 목록 소유자 하나; API의 “추가” 명칭만으로 append 의미를 가정하지 않음 |
| 캐시 query 인코딩 | 일반 query 배열은 422, body 없는 대괄호 query 배열은 400 / `REQUIRED_POST_PARAM_MISSING` | 관찰한 계약에서는 query만으로 수정하지 않음 |
| 캐시 빈 배열 | JSON 빈 레퍼러는 422이며 기존 값 보존 | 관리 리소스의 clear는 미해결; 빈 값이 적용됐다고 state에 기록하지 않음 |
| 캐시 연속 수정 | 바로 다음 PUT은 404 / `NOT_FOUND`와 작업 중 메시지; 부모 서비스는 존재 | 부재로 처리하거나 쓰기를 무조건 재시도하지 않음 |
| 캐시 간격을 둔 수정 | 추가 지연 Read 후 두 차례 교체 성공; 관찰된 상태는 계속 `active` | `active`만으로 쓰기 잠금 해제를 보장할 수 없음; 신뢰할 준비 상태 계약은 미확보 |
| DBMS 상품 | 엔진 버전이 다른 행에 동일 `product_id`가 반복됨; Redis 필터 결과의 유일한 ID로 테스트 | 임의 중복 제거/버전 선택 약속 금지; CPU 단위 미검증 |
| DBMS 생성/조회 | JSON Redis 생성 성공; `waiting` → `active`; 생성 `domain`은 문자열, Read는 object | 생성 응답과 조회 모델 분리; DB 접속 성공의 근거는 아님 |
| DBMS allowip | JSON 배열 전체 교체; 빈 배열은 422이며 기존 IP 보존 | 전체 집합 소유권과 출시 전 빈 집합 미지원 처리 결정 필요 |

출처: [호스팅 서버 조회](https://iwinv-hosting.readme.io/reference/상품-상세-조회),
[캐시 생성](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-생성),
[캐시 레퍼러](https://iwinv-cache.readme.io/reference/레퍼러-추가),
[DBMS 상품](https://iwinv-dbms.readme.io/reference/클라우드-dbms-상품-조회).
링크는 공급사 문서이며, 표는 문서와 다른 실측 결과를 구분합니다.

## 웹메일 조회 지연과 비밀번호 응답 (2026-09-19)

임시 `.invalid` 도메인을 사용했고 DNS 변경이나 메일 발송은 하지 않았습니다.
생성은 HTTP 200, 정수 `service_idx`, `pending`을 반환했지만 첫 서비스 목록은 비어 있었습니다.
생성 ID를 보존하고 기다린 별도 한정 실험에서는 후속 목록에 `active`로 나타났습니다.
따라서 생성 직후 빈 목록만으로 생성 ID를 버리거나 생성을 재시도하면 안 됩니다.

부모 Read는 생성 응답과 달리 `name`을 생략합니다. 하위 계정 생성은 HTTP 200이며 **입력한 비밀번호를 그대로 반환**하고,
`service_idx`는 문자열입니다. 이후 부모 목록에는 계정 ID나 계정 목록이 나타나지 않았습니다.
계정 삭제는 성공 응답을 받았지만 문서에 독립 계정 Read가 없어 부재를 검증할 수 없습니다.
뒤이은 부모 DELETE는 404 / `NOT_FOUND`와 작업 중 메시지를 반환했고 지연 후 부모 목록도 비어 있었습니다.
시간이 지난 뒤 해당 생성 ID로 명시적으로 정리 DELETE를 다시 요청해도 404였고 후속 목록도 비어 있었습니다. 콘솔 해지와 과금 종료는 미확인입니다.
이는 정리/조회 계약의 모호함이며, 부모가 생성되지 않았거나 자식 삭제가 안전하게 부모까지 삭제한다는 증거가 아닙니다.
T040은 진행 중입니다. 계정의 신뢰 가능한 Read/import가 미해결입니다. T041도 진행 중이며 비밀정보 배제 테스트와 별개로 Terraform write-only/state 동작은 아직 구현하지 않았습니다.

내부 `hosted` 디코더는 정수 ID를 float 반올림 없이 보존하고 다른 필드 검증보다 먼저 생성 ID를 추출합니다.
웹메일의 이름 누락을 허용하며, 잘못된/중복 목록과 바뀐 페이지 메타데이터를 거부하고 임의 비밀번호 필드를 결과에 복사하지 않습니다.
`Find` 결과는 해당 목록의 관찰일 뿐이며 서비스별 일관성·삭제 규칙은 별도 필요합니다.
호스팅·캐시·DBMS·웹메일의 비공개 생성/조회 응답 6쌍도 디코딩하고 실제 ID 일치를 대조했으며 payload는 출력하지 않았습니다.
Git에는 합성 fixture만 포함합니다. 이 보조 코드는 호스팅 계열 Terraform 리소스 구현을 뜻하지 않습니다.

출처: [웹메일 생성](https://iwinv-webmail.readme.io/reference/웹-메일-생성),
[계정 생성](https://iwinv-webmail.readme.io/reference/웹-메일-계정-생성),
[서비스 조회](https://iwinv-webmail.readme.io/reference/웹-메일-서비스-조회).

## SSH 키 참조 구현 (2026-09-19)

`iwinv_ssh_keys`, `iwinv_ssh_key`로 C18의 읽기 전용 부분을 구현했습니다.
인증된 API는 문자열 `ssh_key_id`/`name`/`start_date`, 숫자 count/page를 반환했고 total은 없었습니다.
페이지 크기 1에서 기존 키 하나가 첫 페이지를 채웠고 2/3페이지는 일치하는 페이지 번호와 빈 배열을 반환했습니다.
구현은 페이지당 10개를 요청합니다. 실제 Terraform acceptance로 기존 키를 조회하고 후속 무변경 plan을 통과했습니다.
state에는 ID와 이름만 들어갑니다. 키를 생성·다운로드·변경·삭제하거나 서버에 접속하지 않았습니다.

합성 테스트는 여러 full page 뒤 빈 페이지, 이름 중복에도 안정적인 ID 정렬, 없는 ID,
앞 페이지에서 ID를 찾은 뒤 발생한 후속 오류, 잘못된 메타데이터, 중복 ID, 응답 메타데이터 변화,
취소, 페이지 상한, 빈 Terraform 컬렉션, 알 수 없는 키 본문 필드 배제를 검증합니다.
단건 Data Source도 모든 페이지를 검증한 뒤 정확한 ID를 반환하며 이름으로 고르거나 없는 상세 API를 만들지 않습니다.
T006/T009/T010의 추가 근거일 뿐 다른 API 계약까지 완료한 것은 아닙니다.
키 생성·삭제와 서버 SSH 설치는 미해결 또는 미검증이며 관리 키 리소스 지원을 주장하지 않습니다.

출처: [SSH 키 목록](https://iwinv-common.readme.io/reference/get_new-endpoint-1-1).

## 보안 그룹 타입 어댑터와 계약 보정 (2026-09-19)

내부 network 서비스에 그룹 목록·상세·생성·수정·삭제 계약을 구현했습니다. P1 계약 준비이며,
Terraform 그룹/규칙/연결 리소스를 등록하지 않았고 P2/P3 완료 조건도 열려 있습니다.

첫 어댑터 실환경 실행은 **생성 전** 목록 검증에서 실패했습니다. 이전 구조 요약이 빠뜨린 `page_no`/`page_size`가 있었습니다.
기본 페이지 크기는 50이었고, 빈 목록에서 명시한 페이지 크기 1과 페이지 번호 1/2가 그대로 반환됐습니다.
데이터가 있는 어댑터 목록 조회는 페이지 크기 50으로 검증했습니다. 상세/생성/수정은 count만 있고 페이지 정보가 없습니다.
목록 구현은 페이지를 순회·검증하고 중복/후속 오류를 거부하며 최대 1,000페이지로 제한합니다.
full page와 종료는 합성 테스트로 검증했으며 실제 그룹이 여러 페이지인 계정은 아직 검증하지 않았습니다.
추출한 OpenAPI에는 이 경로의 페이지 인자가 없어 공급사 문서 사실을 바꾸지 않고 실측 근거로 구분합니다.

| 계약 | 근거와 구현 |
| --- | --- |
| 그룹 ID | 정확한 `FIREWALL-…` 경로 조각 검증; 생성 응답에 ID 하나가 있으면 다른 필드/count/status 검증 실패에도 ID 보존 |
| 상세 부재 | HTTP 200 + 빈 배열 + count 0만 부재로 판단; 404·업무 오류·다른 ID·잘못된 응답은 오류 유지 |
| 설명 전송 | JSON으로 입력 원문 전송. 설명 응답만 HTML 이스케이프되어 정확히 한 번 디코딩하며 이름은 그대로 보존 |
| 인코딩 실험 | 따옴표·꺾쇠·앰퍼샌드·문자 그대로의 `&amp;`/`&#39;`/`&lt;`·한글·분해형 유니코드가 설명 1회 디코딩 후 왕복. URL 인코딩은 문자 그대로 저장돼 적용하면 안 됨 |
| 빈 설명 수정 | 앞선 ASCII 테스트에서는 기존 값 유지. 이스케이프된 설명의 후속 실험에서는 보존 실패했으므로 무해한 무동작으로 보지 않으며 어댑터가 전송 전 거부 |
| 설명 생략 수정 | 한정 API/Go 테스트에서 설명을 유지하며 이름·ICMP 변경 성공. 생략은 초기화와 다름 |
| 설명 없는 생성 | 탐색 요청에서 사용 가능한 생성 응답을 확보하지 못했고 당시 하네스는 오류 분류를 보존하지 못함. 이후 목록은 비었음. 필수 인자라는 증거는 아니며 생략 계약은 미해결 |
| 삭제 | 한 번 요청한 성공 응답은 접수만 검증. 별도 정확한 ID 상세 Read로 부재 확인 |

페이지와 설명 디코딩을 보정한 뒤 Go 어댑터 실환경 테스트에서 생성 응답, 목록/상세, 한글·특수문자 수정,
clear 거부 후 기존 값 보존, 설명 생략 상태의 이름/ICMP 수정, 삭제 후 부재를 통과했습니다.
ID를 기록한 Go 테스트 그룹 3개와 인코딩 실험 그룹 2개는 삭제를 확인했습니다.
ID 없는 생성 실패는 이름으로 채택하거나 자동 재시도하지 않았으며 후속 목록에서 해당 테스트 이름도 발견되지 않았습니다.
원본 응답·실제 ID·정리 대장은 비공개로 보관합니다. 합성 테스트는 부분 생성 ID 보존, 모호한 결과 개수,
잘못된 경로, 취소, 페이지 상한, null/빈 값/누락과 오류 시 1회 요청도 확인합니다.
하네스는 이제 안전한 생성 오류 분류를 보존하고, 확보한 생성 ID의 정리를 등록한 후 대장에 저장합니다.

T005/T006/T009/T010과 C15의 추가 근거이며 Terraform state/import·연결·규칙 cascade acceptance는 아닙니다.
출처: [그룹 목록](https://iwinv.readme.io/reference/get_v1-security-groups),
[그룹 상세](https://iwinv.readme.io/reference/get_v1-security-groups-id),
[그룹 생성](https://iwinv.readme.io/reference/post_v1-security-groups).

## 첫 관리 리소스: 보안 그룹 속성 (2026-09-19, T057)

`iwinv_security_group`을 개발용 리소스로 등록했습니다. 이름, 비어 있지 않은 설명, ICMP를 지원하며,
정확한 ID import와 작업별 timeout을 제공합니다. 범위와 복구 절차는 [한·영 리소스 가이드](../../docs/ko/resources/security_group.md)에 있습니다.
서버 생성이 차단된 동안 독립적인 네트워크 기능을 진전시킨 것이며, P1/P2/P3나 구현 이슈 6개를 완료 처리하지 않습니다.

설명 기본값을 결정하기 전 content를 생략/빈 값으로 지정한 진단 생성 2회는 HTTP 403 / `CHECK_IP`,
중첩된 일시 장애 메시지와 함께 ID 없이 끝났습니다. 후속 목록에서 두 요청 이름은 발견되지 않았습니다.
프로세스의 출발 IP는 허용 IP와 일치했고, 비어 있지 않은 설명의 대조군은 생성 성공 후 삭제 및 정확한 ID 부재를 확인했습니다.
이는 한정된 관측이며 `CHECK_IP`의 일반 의미를 다시 정의하지 않습니다. 공식 오류 대장은 IP 제한으로 설명합니다.
따라서 Provider는 검증된 비어 있지 않은 설명을 전송하고(HCL 생략 시 `Managed by Terraform`), 명시적인 빈 설명을 거부합니다.
이전 설명 생략 관측에는 오류 분류가 보존되지 않았으므로 이번 결과를 소급 적용하지 않습니다.

Go 1.26.1 / Terraform 1.14.2로 실환경 Terraform 실행 2회가 통과했습니다. 반환된 생성 ID 4개를 비공개 기록했고 모두 정확한 ID의 부재를 확인했습니다.
두 번째 실행은 원격 객체 삭제 없이 Terraform 소유권을 해제하고 같은 영속 state로 재import한 뒤 무변경 plan까지 추가 확인했습니다.
두 실행 모두 ID를 유지한 생성/조회/수정, 전체 속성 import 비교, 한글/리터럴 엔티티 설명,
무변경 plan, 외부 변경 감지/복원, 외부 삭제/재생성과 최종 destroy를 검증했습니다. 기존 그룹은 변경하지 않았습니다.
위 설명 대조군은 Terraform 생성 4개와 별개이며 역시 삭제했습니다.
비공개 대장과 원본 state/로그는 저장소에 포함하지 않으며 공개 fixture는 합성 ID만 사용합니다.

합성 Terraform CLI 테스트도 기본값, API 쓰기가 없는 timeout만 변경, 빈 설명 거부,
잘못된 생성 응답으로 apply가 실패해도 ID를 유지하여 두 번째 POST 없이 destroy하는 경우를 통과했습니다.
직접 Framework 테스트는 생성 반영 지연/기한 초과, 수정/삭제 실패 시 이전 state 보존,
HTTP 인증/부재/요청 제한/서버 오류와 엔드포인트별 확정 부재를 검증합니다.
근거는 `internal/provider/security_group_resource_test.go`, `internal/provider/security_group_live_test.go`와 네트워크 어댑터 테스트입니다.

구현 대장에는 이 리소스의 POST/상세 GET/PUT/DELETE만 등록했습니다. 내부/테스트에서 쓰는 그룹 목록은 Terraform 기능으로 표시하지 않습니다.
규칙, 연결, 패킷 동작, API 길이 경계, 실제 전체 페이지 경계와 과금 종료는 실환경 검증 범위에 포함되지 않습니다.
서버 존 제한과 이전 웹메일 콘솔/해지 불확실성은 계속 미해결입니다. 이번 그룹 정리가 이전의 모든 서비스 정리를 증명하지는 않습니다.

## 독립 ingress/egress 규칙 (2026-09-19, C16, T058)

`iwinv_security_group_ingress_rule`과 `iwinv_security_group_egress_rule`을 개발용 리소스로 등록했습니다.
소유권, 정확한 복합 import ID, 교체와 복구는 [공통 가이드](../../docs/ko/guides/security_group_rules.md)에 정의했습니다.
P2 서버와 P3 전체 완료 조건은 계속 열어 둡니다. 규칙 지원이 서버 연결·스토리지·패킷 필터링 검증을 의미하지 않습니다.

추가 API 직접 실험으로 다음의 한정된 계약을 확인했습니다.

- 규칙의 `title`/`content`는 한글과 리터럴 HTML 엔티티를 포함해 원문으로 반환합니다. 그룹 설명의 HTML 디코딩을 재사용하면 안 됩니다.
- 빈 설명과 생략한 설명으로 생성하면 null을 반환합니다. 빈 값/null 수정과 설명 생략 수정은 기존 설명을 유지합니다.
- 방향, 프로토콜, 포트 범위, CIDR, 이름과 비어 있지 않은 설명은 정수 규칙 ID를 유지하며 제자리 수정됐습니다.
- 완전히 같은 규칙의 중복 생성은 HTTP 400 / `CHECK_PARAM`입니다. 소문자 `tcp`와 `inbound`를 각각 별도로 시험해 `CHECK_PARAM_ENUM`을 확인했습니다.
- IPv6는 `IPV6_NOT_SUPPORTED`, `ICMP`는 `CHECK_PARAM_ENUM`입니다. 포트 `0`과 `0-65535`는 `CHECK_PARAM`, `65535`는 성공했습니다.
- bare IP 수정은 `CHECK_PARAM`으로 실패했고 나머지 입력이 같은 CIDR 수정은 성공했습니다. IPv4 CIDR 입력만 지원합니다.
- 규칙 53개 fixture의 기본 목록과 `page_no=1/2,page_size=1` 조회 모두 기록된 ID 전체를 반환했습니다. 페이지 메타데이터는 없고 쿼리 페이지 인수는 무시됐습니다.
- 초기 직접 실험에서 harness의 null 설명 가정 오류와 bare IP 수정 거부를 발견했습니다. 중단된 두 실행 모두 기록한 자식과 부모를 정리했으며 완전한 실험 성공으로 표시하지 않습니다.

타입 어댑터는 정확한 ID를 찾기 전에 전체 행을 검증하고, 페이지/count 계약 변경을 거부하며, float64 없이 int64 ID를 보존합니다.
생성 응답의 나머지 필드 검증이 실패해도 확보한 ID는 유지합니다. 규칙 Read는 부모 상세를 먼저 읽습니다.
부모의 성공/빈 결과로 부모 부재를 판단하며 규칙의 `CHECK_PARAM`이나 404만으로 state를 지우지 않습니다.
쓰기는 자동 재전송하지 않으며, 빈 설명 수정 요청은 I/O 전에 거부합니다.

Go 어댑터 실환경 테스트에서 생성/조회/전체 수정/설명 생략 수정/다른 규칙을 보존한 개별 삭제와 정리를 통과했습니다.
Go 1.26.1, Terraform 1.14.2와 race detector를 사용한 실환경 Terraform acceptance에서 두 방향 리소스,
두 방향의 전체 import 비교·영속 재import 후 무변경 plan·방향 drift 복원·설명 초기화 교체와 ingress의 부모 변경 교체,
외부 규칙 삭제/재생성, 자식 먼저·부모 나중 destroy를 통과했습니다. 소유권 wrapper는 각 자식 부재 확인 전 부모 삭제를 거부합니다.
직접 실험/Go/Terraform 실행 전체에서 부모 7개와 개별 ID를 기록한 규칙 66개를 모두 삭제했으며, 부모가 있을 때 자식 부재를 확인했습니다.
비공개 응답, ID, state와 로그는 Git에 포함하지 않습니다. 이번 정리는 이전 웹메일 해지/과금 질문을 해결하지 않습니다.

합성 Terraform 테스트는 POST 추가 없이 생성 실패 ID 정리, update 요청이 없는 timeout만 변경,
부모 생성 전 잘못된 입력 거부, 부모 부재 처리, 수정/삭제 실패 state 보존,
기존 설명이 비어 있지 않고 새 설명이 unknown일 때 보수적인 교체 계획까지 추가로 통과했습니다.
알려진 비어 있지 않은 값은 제자리 수정하며 unknown 때문에 apply에서 승인하지 않은 교체를 새로 추가하지 않습니다.
그룹과 규칙 모두 쓰기 시점에 unknown인 timeout을 미리 거부하여 잘못된 state가 반환된 생성 ID를 무효화하지 않게 했습니다.

근거: `internal/services/network/rules_test.go`, `internal/client/rules_live_test.go`,
`internal/provider/security_group_rule_resource_test.go`, `internal/provider/security_group_rule_live_test.go`.
패킷 동작, 연결, 전체 경계 변형과 물리적인 연쇄 삭제를 모두 검증하지 않았으므로 T031/T032는 부분 검증 상태를 유지합니다.
공식 계약: [목록](https://iwinv.readme.io/reference/get_v1-security-groups-id-rules), [생성](https://iwinv.readme.io/reference/post_v1-security-groups-id-rules),
[수정](https://iwinv.readme.io/reference/put_v1-security-groups-id-rules-rule-id), [삭제](https://iwinv.readme.io/reference/delete_v1-security-groups-id-rules-rule-id).

## 타입이 있는 호스팅 어댑터와 교체 제약 (2026-09-19)

상품/서버 카탈로그, 서비스 전체 목록의 정확한 ID 선택, JSON 생성과 삭제 접수 어댑터를 추가했습니다.
Terraform 리소스나 Data Source는 등록하지 않았습니다. C19/C29, T037/T041의 부분 근거이며
T059는 아래의 어댑터 범위로만 통과했습니다. T038과 비밀번호 plan/state 검증은 아직 미완료입니다.

- 전체 상품 목록과 SHARE/SINGLE 필터의 분할, 필수 product_id를 보낸 PHP 8.4 서버 선택을 확인했습니다.
- 첫 실험에서 기본 도메인 서비스는 생성·active·삭제·부재 확인에 성공했지만 두 번째 요청이 실패했습니다.
  입력을 분리한 실험에서 명시적 빈 description이 HTTP 422/errors.description으로 거부되는 것을 확인했습니다.
  설명 생략은 빈 문자열 Read로 이어집니다. 어댑터는 빈 설명 전송을 사전에 거부하고 생략과 구분합니다.
- 방화벽 N과 사용자 `.invalid` 도메인은 별도 실험에서 생성·active·삭제 확인에 성공했습니다.
  재실행한 Go race 계약 테스트는 두 서비스를 함께 생성하고 각각 Y/N, 기본/사용자 도메인,
  생략/한글·리터럴 `&amp; + %` 설명을 확인했습니다. 이름과 설명을 HTML 디코딩하지 않습니다.
- 사용자 도메인을 1개 요청한 서비스 응답은 기본 도메인까지 총 2개 매핑을 포함했습니다.
  입력 map을 Read map으로 그대로 대체하면 불일치가 생길 수 있어 [수명주기 설계](webhosting-lifecycle.md)에서 소유권을 분리합니다.
- 두 생성 ID가 전체 목록에 함께 나타났으며 하나의 삭제 후 다른 서비스는 active로 유지됐습니다.
  이번 단계에서 만든 총 5개 호스팅 모두 정확한 생성 ID를 비공개 기록하고 HTTP 200 삭제 접수와 후속 부재를 확인했습니다.
  처음 실패한 시도의 두 계정명도 후속 목록에 없었으며, 이름 기반 채택이나 생성 자동 재전송은 하지 않았습니다.
- 합성 테스트는 2^53 초과/int64 ID, 부분·중복·잘못된 목록, HTTP/메타데이터 변경, nullable 설명,
  미확인 생성 응답 후 ID 보존, 안전한 진단과 입력 인코딩을 검증합니다. 가격·부가세와 저장소 단위는 모델에서 제외했습니다.

C29의 24시간 재사용 금지는 공식 삭제 문서에서 확인한 제약이며 24시간 경과 재생성 실측은 아닙니다.
호스팅의 Terraform import·교체·외부 drift·비밀번호 비저장, 실제 HTTP/FTP/DB 접속, 과금 종료는 아직 검증하지 않았습니다.
이번 호스팅 정리 결과는 앞선 웹메일 콘솔 해지/과금 종료 미확정 상태를 해결하지 않습니다.

출처: [호스팅 삭제](https://iwinv-hosting.readme.io/reference/웹-호스팅-삭제),
[호스팅 생성](https://iwinv-hosting.readme.io/reference/웹-호스팅-생성).

## 웹메일 API 부재와 콘솔 잔존의 불일치 (2026-09-19)

인증된 콘솔에서 앞선 테스트 웹메일의 생성 ID·이름·도메인을 비공개 기록과 대조했습니다.
동일 서비스가 목록에는 **작업중, 계정 수 0**, 상세에는 **운영중**으로 남았고 삭제 버튼은 비활성 상태였습니다.
같은 시점의 `GET /v1/webmail`은 HTTP 200/SUCCESS 빈 배열을 반환했습니다.
따라서 이 서비스에서 성공한 빈 API 목록은 삭제/해지 완료의 증거가 될 수 없습니다.
콘솔 경로를 Provider에서 호출하거나 UI의 비활성 제어를 우회하지 않습니다.

T056은 이 미정리 테스트 서비스 때문에 `failed`로 표시했습니다. 다른 테스트의 확인된 삭제 근거를 취소하는 것은 아니지만
전체 정리 완료 조건은 충족하지 못했습니다. C23/T040의 신뢰 가능한 Read 계약도 여전히 미해결입니다.
테스트 서비스만 해지하고 과금 종료와 API 누락 사유를 확인하는 공급사 문의 초안을 비공개로 준비했습니다.
외부 메시지는 아직 보내지 않았으며 별도 사용자 승인을 요청했습니다.
실제 ID, 계정명·도메인, 로그인 정보와 콘솔 원문은 공개 저장소에 포함하지 않습니다.

## Terraform 호스팅 수명주기 (T060, 2026-09-19)

개발 Provider에 `iwinv_webhosting`을 등록했습니다. 앞선 내부 어댑터 전용 단계의 제한은 이 리소스에 한해 해소했으며
상품/서버 카탈로그는 내부용입니다. Go 1.26.1, Terraform 1.14.2, `go test -race` 실환경 acceptance가 62.53초에 통과했습니다.
사용자 도메인을 지원하는 SHARE 상품과 PHP 8.4 서버를 선택했습니다. 입력·정확한 ID·요청 의도/응답 대장·로그는 비공개이며
기존 서비스는 변경하지 않았습니다.

이번에 생성한 서비스 ID 4개 모두 삭제 응답과 정확한 ID의 부재를 확인했습니다.
두 서비스를 함께 유지하며 초기 무변경 plan과 전체 읽기 속성 import를 검증한 뒤, 사용자 `.invalid` 도메인·원문 설명·방화벽 N인
새 계정을 먼저 만들고 이전 계정을 삭제했습니다. 다른 서비스는 active로 보존했습니다.
삭제 없이 상태 소유권을 해제한 뒤 생성 서버/비밀번호/버전 없이 실제 상태에 재import하고 Read·무변경 plan을 확인했습니다.
외부 삭제 감지 후 새 계정 재생성과 최종 정리도 통과했습니다.

파싱된 plan뿐 아니라 실제 압축된 저장 plan 내용과 state에서 ephemeral 초기 비밀번호가 없는지 검사했습니다.
합성 Core 테스트는 잘못된 생성 응답 후 ID 보존·정리, 새 계정의 삭제 우선 교체, 변경값 unknown, 교체 입력 누락,
도메인 drift와 타임아웃만의 로컬 갱신을 통과했습니다. taint/명시적 `-replace`는 파괴적인 plan만 확인하고 같은 이름 재생성은
수행하지 않았습니다. 직접 리소스 테스트는 숨겨진 생성, 대기 만료, API 오류와 삭제 실패 시 상태 보존을 검증합니다.
일반 교체에는 확정된 새 계정명·서버·초기 비밀번호를 요구합니다.

이 범위로 T060을 통과 처리합니다. 근거는 `internal/provider/webhosting_resource_test.go`, `internal/provider/webhosting_live_test.go`입니다.
다른 서비스까지 포함한 T038/T041은 부분 완료이며, 콘솔에 남은 기존 테스트 웹메일 때문에 T056은 실패 상태를 유지합니다.
이번 호스팅 4개 정리를 전체 정리 완료로 보지 않습니다. 데이터 연결·이전·다른 상품/버전과 과금 종료는 미검증입니다.

## 호스팅 카탈로그 Data Source (T061, 2026-09-19)

앞서 검증한 디코더로 `iwinv_webhosting_products`, `iwinv_webhosting_servers`를 등록했습니다.
상품 필터는 생략/SHARE/SINGLE을 지원하고 서버 조회는 정확한 상품 ID를 요구합니다. 서버 정수 ID는 float64 변환 없이
문자열로 보존합니다. 두 Data Source 모두 ID순으로 정렬한 완전한 타입 결과만 제공하고 생성 상품/서버를 자동 선택하지 않습니다.
가격·VAT·저장공간/트래픽 단위는 계속 제외합니다.

Go 1.26.1/Terraform 1.14.2 실환경 읽기 acceptance가 18.40초에 통과했습니다. 세 상품 필터, 상품별 서버 조회,
후속 무변경 plan을 검증했고 서비스 생성·변경·삭제는 없었습니다. 로그는 비공개입니다.
합성 Core 테스트는 2^53 초과 ID, 쿼리 보존, 표시 텍스트 원문, PHP 라벨 정렬, 선택 문자열의 빈 값, 빈 배열을 확인했으며
잘못된 입력은 요청 전에 거부했습니다. 중복/잘못된 배열, 변경된 페이지 메타데이터와 API 오류는 두 Data Source 모두
부분 결과 대신 실패로 처리합니다. 카탈로그 계약 범위로 T061을 통과 처리하며 생성 가용성과 다른 상품/버전 수명주기는 별도입니다.
근거: `internal/provider/webhosting_catalog_data_source_test.go`.

## DBMS 어댑터와 Terraform 수명주기 (T062/T063, 2026-09-19)

최초 어댑터 실행은 생성 전 카탈로그 검증에서 멈췄습니다. 전체 113행 중 available인데 상품 ID가 빈 문자열인 행이 4개였습니다.
버전 간 상품 ID 중복과 함께 C30에 기록합니다. 빈 ID 행을 검토용으로 보존하되 null/누락은 거부하고 빈 ID 생성은 차단합니다.
버전 선택값을 만들거나 조용히 중복 제거하지 않습니다. 수정한 어댑터 실행은 모호하지 않은 비어 있지 않은 ID의 STD Redis 상품을
선택해 75.17초에 통과했습니다. 신규 두 서비스가 active가 됐고 생략한 설명은 빈 문자열, 원문 설명은 HTML 디코딩 없이 왕복했습니다.
두 IP의 JSON 수정으로 첫 서비스의 목록 전체를 바꾸고 다른 서비스를 보존했습니다. 두 ID 모두 삭제 응답과 부재를 확인했습니다.

Terraform 1.14.2 / Go 1.26.1 race acceptance가 180.69초에 통과했습니다. 두 리소스를 함께 유지하면서 새 계정 교체와
외부 삭제 복구를 포함해 총 4개의 서비스 ID를 생성했습니다. 허용 집합 제자리 수정은 ID를 유지했고 외부 drift를 복구했습니다.
읽기 속성 전체 import, 초기 계정 이력 없는 영속 재import와 무변경 plan을 통과했습니다.
새 계정 create-before-destroy가 다른 서비스를 보존했고 최종적으로 4개 ID 전부 삭제 응답/부재를 확인했습니다.
어댑터와 Terraform 테스트의 DBMS 합계 6개 모두 정리했으며 기존 DBMS·DB 내용·네트워크 연결·DNS·메시지 수신자는 변경하지 않았습니다.
요청 의도·응답·식별자·Terraform state·로그는 비공개로 보관합니다.

합성 테스트는 2^53 초과 ID, 생성/목록 도메인 형식 차이, 잘못된/부분 목록, 생성 실패 ID의 Core 정리,
unknown 값, 잘못된/빈/CIDR/IPv6 허용 목록, 순서 무관 집합, 수정/삭제 오류·대기 만료 시 상태 보존,
숨겨진 생성 식별자, 보수적 교체 guard와 명시적 교체 plan을 검증합니다. 같은 계정의 명시적 교체는 plan만 확인하고 적용하지 않습니다.
계정명은 Read에 없는 생성 이력이므로 import에서 도메인으로 추측하지 않습니다.

문서화한 STD Redis 제어 API 범위로 T062/T063을 통과 처리합니다. 다른 상품/서비스를 포함한 T037/T038/T039는 부분 완료이며,
이전 웹메일로 인한 T056 실패는 유지합니다. 데이터 접속, 백업/이전, 다른 엔진, 계정명 재사용 시점과 과금 종료는 미검증입니다.
DBMS 카탈로그 조회는 내부용이며 Data Source를 등록한 것은 아닙니다. [설계 결정](dbms-lifecycle.md)을 함께 확인하세요.

## DBMS 상품 Data Source (T064) — 2026-09-19

`iwinv_db_instance_products`를 등록했습니다. 선택적인 정확한 `product_type`·`engine` API 필터와 정렬된 전체 상품 행을 제공합니다.
빈 생성 ID와 버전 간 중복 ID를 보존하며(C30), 누락/null ID·잘못된 행·중복 조합·부분 메타데이터·unknown/잘못된 필터·API 오류는 거부합니다.
상품을 자동 선택하거나 생성 시 버전을 선택할 수 있다고 가정하지 않습니다.

`TestAccDBProducts`는 Terraform 1.14.2/Go race에서 49.77초로 통과했습니다. 전체 조회, 엔진 필터 6개, 등급 필터 2개,
STD/redis 조합과 후속 무변경 plan을 확인했습니다. 필터 접수·메타데이터 검증이며 엔진 식별이나 상품 생성을 독립 검증한 것은 아닙니다.
합성 Core는 빈/중복 ID 보존·정렬·오류·잘못된 입력 차단을 검증합니다. 새 리소스 생성이나 수정은 없습니다.
T062/T063 정리 후 콘솔에서도 검색 필터 없는 DBMS 목록이 비어 있음을 독립 확인했습니다.
현재 개발 Provider는 Data Source 10개와 관리 리소스 5개입니다. 전체 T038, 웹메일 T056과 Registry 릴리스는 미완료입니다.

## 캐시 어댑터와 작업중 거절 (T065) — 2026-09-19

캐시 제어 API 5개 작업의 타입 어댑터, null 상품 ID와 정확한 서비스 ID를 추가했습니다. C31은 카탈로그 SHARE와 서비스 SINGLE의
불일치를 기록하며 값을 바꾸거나 격리 수준을 보장하지 않습니다. 생성은 검증한 JSON pw.FTP를 보내고 무시되는 생성 시 리퍼러는
제외합니다. 비밀번호는 타입 입력 대장이나 읽기 모델에 직렬화하지 않으며 Terraform plan/state write-only 비저장은 아직 별도 검증입니다.

첫 실환경 2개 서비스 실행은 89.81초로 통과했습니다. PUT 작업중 거절 한 번과 기존 목록 보존 Read 이후 교체 접수를 확인했습니다.
두 번째 2개 서비스 실행은 삭제 전 45초 fixture 대기를 제거하고 45.49초로 통과했습니다. PUT·DELETE에서 각각 정확한 작업중 거절을
한 번씩 받았으며 대상이 그대로임을 확인한 뒤 제한된 재시도가 성공했습니다. 공통 클라이언트는 정확한 메서드/경로/상태/code/message/result
조합만 분류하고 서버 원문을 노출하거나 자동 재시도하지 않습니다.

새 캐시 ID 총 4개 모두 삭제 응답과 정확한 ID 부재를 확인했으며 첫 실행 후 콘솔도 검색 필터 없이 비어 있었습니다. 기존 인프라는
변경하지 않았습니다. Core 리소스/import/drift/교체/비밀 state·복구, 상품 Data Source, tenant API, 다른 상품, 실제 컨텐츠 접속과
과금은 남아 있습니다. [캐시 설계](cache-lifecycle.md)를 참고하세요. T038과 웹메일 T056은 미완료이며 공개 등록은 Data Source 10개·리소스 5개입니다.

## 캐시 Terraform 수명주기 (T066) — 2026-09-19

`iwinv_content_cache`를 등록했습니다. 전체 리퍼러 집합 소유, write-only 초기 FTP 비밀번호, 로컬 비밀번호 버전 교체 신호,
정확한 ID import, 비어 있지 않은 집합 초기화 시 새 계정 교체를 제공합니다. 이름·설명·상품·계정 변경도 교체입니다.
정확히 분류한 PUT/DELETE 작업중 거절만 부모 전체 보존을 재조회한 뒤 재시도하며 불확실한 쓰기는 반복하지 않습니다.

`TestAccContentCache`는 Terraform 1.14.2/Go race에서 116.04초로 통과했습니다. 공존, 빈/비어 있지 않은 초기 목록,
ID 보존 수정, 외부 drift 복원, 전체 조회 필드 import, 비밀번호·버전 없는 재import·무변경 plan, 새 계정의 빈 집합
create-before-destroy, 버전·설명 교체와 외부 삭제·재생성을 검증했습니다. 저장된 압축 plan/state에 ephemeral 비밀번호가 없습니다.
PUT 작업중 거절 한 번을 처리했으며 DELETE 작업중은 이번 실행에서 발생하지 않아 별도 T065 실환경과 합성 Core 근거를 사용합니다.
합성 테스트는 설정 실패 정리, 지연·잘못된 생성 응답, unknown 교체 plan, taint·명시적 교체, 오류 시 state 보존도 검증합니다.
새 ID 5개 모두 삭제 접수와 정확한 부재를 확인했고 새로고침한 콘솔의 검색 필터 없는 캐시 목록도 비어 있었습니다.
기존 인프라는 변경하지 않았습니다. [캐시 설계](cache-lifecycle.md)와 한영 리소스 가이드에 파괴적 변경을 설명합니다.

개발 Provider는 Data Source 10개와 관리 리소스 6개입니다. 캐시 상품 Data Source, 다른 상품, tenant/content API,
FTP 접속, 과금, 전체 T038, 웹메일 T056과 Registry 릴리스는 미완료입니다.

## 캐시 상품 조회 (T067) — 2026-09-19

`iwinv_content_cache_products`를 등록했습니다. null·빈 상품 ID를 보존하고 API 순서와 무관하게 전체 행을 정렬합니다.
정확한 SHARE/SINGLE 필터에서 Read 시 unknown·빈 문자열·대소문자 오류를 거절합니다. ID 누락·잘못된 타입·중복,
부분 메타데이터·잘못된 행·필터 불일치·API 오류는 전체 Read를 실패시킵니다. 정렬로 생성 상품을 추정하지 않습니다.

`TestAccCacheProducts`는 Terraform 1.14.2/Go race에서 13.37초로 통과했습니다. 전체/SHARE/SINGLE 조회, 실제 null ID와
무변경 plan을 검증했습니다. 합성 Core는 null·빈 문자열 구분과 API 반환 순서 변경도 검증합니다. 새 서비스를 생성하지 않았습니다.
개발 지원은 Data Source 11개·관리 리소스 6개이며 다른 상품, tenant/content API와 Registry 릴리스는 남아 있습니다.

## NAS 타입 어댑터와 준비 상태 (T068) — 2026-09-19

새 NAS 준비 상태 실험은 pending 이후 조회 시작 7.12초에 active를 관측했습니다. 100 GB·설명 원문·초기 RO 권한이 일치했고,
RW/RO 맵 교체 후 삭제 접수·정확한 ID 부재를 확인했습니다. sharename이 Read에 없다는 점도 확인했습니다.
이 시간은 지연 보장이 아니며 mount 정보에서 공유 이름을 추정하지 않습니다.

이어 `TestAccNASControlPlaneWrites`가 Go race에서 25.82초로 통과했습니다. 카탈로그 최소 100 GB의 api_nas 두 서비스 공존,
active 대기, 설명 원문/생략, 호스트 2개의 전체 권한 맵 교체, 기존 호스트의 RW→RO 변경과 다른 호스트 제거, 다른 서비스 보존,
삭제 접수·정확한 ID 부재를 검증했습니다. 선행 실험을 포함한 새 NAS 3개를 모두 정리했습니다.
독립적으로 불러온 /na 콘솔의 검색 조건 없는 목록도 비어 있었고 가이드 링크가 api-nas 서비스임을 확인했습니다.

타입 스키마는 정확한 int64 ID, 카탈로그 빈 ID·null 버전·coming-soon의 0 범위, 불투명한 mount 문자열을 보존합니다.
수정 응답은 ip/acl 객체 배열이며 생성·Read의 IP→권한 맵과 별도로 디코딩합니다. 합성 테스트는 부분 생성 ID 보존,
잘못된 입력, 전체 목록, 권한·중복 응답, 오류, 쓰기 비재시도, 인증정보·생성 이력 제외를 검증합니다.
[NAS 수명주기 설계](nas-lifecycle.md)를 참고하세요. NAS Terraform 등록·import·교체, NFS/파일, tenant API, 과금,
전체 T038과 별도 웹메일 정리 실패 T056은 미완료입니다. 기존 인프라는 변경하지 않았습니다.

## NAS Terraform Core acceptance (T069), 2026-09-19

`TestAccSharedStorage`가 Terraform 1.14.2·Go race에서 82.29초로 통과했습니다. 새 api_nas 두 개가 100 GB로 공존했고,
ID를 유지하는 전체 RO/RW 맵 교체, 외부 권한 drift 복구, 모든 조회 속성의 import, `share_name` 없는 영속 import·무변경 plan,
그 import 이후 권한 수정을 검증했습니다. 새 공유 이름의 create-before-destroy 교체로 200 GB를 생성하고 한글 설명 원문을
보존했으며 후속 plan도 무변경이었습니다. 외부 삭제 후 새 공유 이름으로 재생성도 통과했습니다.
생성한 4개 ID 모두 삭제 접수·검증된 정확한 ID 부재를 확인했습니다. 별도로 새로고침한 /na API NAS 콘솔은 검색어가 비어 있고
목록도 비어 있었습니다. 기존 서비스는 변경하지 않았습니다.

합성 Core 검사는 맵 순서, unknown 권한·용량, 잘못된 입력 거절, 2^53보다 큰 정확한 ID, 생성 실패 ID 보존·정리,
검증 전 ID 부재, 쓰기 전 동시 변경 감지, 조회·수정·삭제 실패, 대기 timeout과 쓰기 자동 반복 금지를 다룹니다.
명시적인 taint/`-replace`는 plan만 검증했으며 같은 공유 이름 재생성을 보장하지 않습니다.
용량 변경은 데이터 삭제를 수반하는 교체이며 resize·이전이 아닙니다. 권한만 제자리 수정합니다.
import에서 읽을 수 없는 공유 이름 이력은 생략하고 mount 정보는 관측 문자열로 보존합니다.
NFS·파일 접근·실제 권한 강제·tenant API·백업·다른 상품·과금 종료는 미검증이며 전체 T038과 별도 웹메일 정리 T056은 미완료입니다.

개발 지원은 Data Source 11개와 관리 리소스 7개입니다. NAS 상품 조회는 내부 어댑터입니다.
[리소스 가이드](../../docs/ko/resources/shared_storage.md)와 [수명주기 설계](nas-lifecycle.md)를 참고하세요. Registry 릴리스는 아직 없습니다.

## NAS 상품 Data Source (T070), 2026-09-19

`iwinv_shared_storage_products`는 전체 상품을 ID·이름 순으로 정렬하며 준비 중인 상품의 빈 ID·null 버전·용량 0을 보존합니다.
최소·최대 용량은 API 문서상 GB이며 제자리 resize 지원이나 독립적인 버전 선택을 뜻하지 않습니다.
기본 상품을 자동 선택하거나 문서에 없는 필터를 만들지 않습니다.

`TestAccStorageProducts`는 Terraform 1.14.2·Go race에서 무변경 plan을 포함해 3.13초로 통과했습니다.
100–2000 GB의 available api_nas와 빈 ID·null 버전·용량 0인 생성 불가 행을 확인했으며 읽기 전용 실행입니다.
합성 Core에서는 응답 순서 반전, null·빈 버전 구분, 이름 원문 보존, 빈 배열 수용과 API 오류·누락/null/잘못된 ID 타입·
버전 누락/잘못된 타입·용량 누락/음수/소수/역전·식별자 중복·메타데이터 변경 거절을 검증합니다.
상품 노출만으로 모든 상품·가격·파일·과금을 검증하지 않습니다.

개발 지원은 Data Source 12개·관리 리소스 7개입니다. 문서화된 NAS control-plane 5개 작업 모두 등록된 구현으로 연결했습니다.
별도 tenant/NFS 기능, 전체 T038·웹메일 정리 T056·Registry 릴리스는 미완료입니다.
[상품 가이드](../../docs/ko/data-sources/shared_storage_products.md) · [NAS 수명주기](nas-lifecycle.md).

## 청구 조회 계약 (C32/T071), 2026-09-19

읽기 전용 실측에서 정수 KRW, 페이지별 `count`, 문자열 페이지·크기 메타데이터와 크기 1/2 페이지의 일치를 확인했습니다.
사용 시작일이 다른 행을 포함해 날짜 필터가 청구일에 적용되고 같은 날짜·금액의 경계를 포함함을 확인했습니다.
부가세 제외 금액 필터를 전체 타입 행과 대조합니다. 빈 필터 결과는 정확한 HTTP 400 `EMPTY_SET`, 끝을 지난 offset은
HTTP 200·빈 배열이었습니다. 목록 첫 페이지 EMPTY_SET만 빈 결과로 처리하고 중간 오류는 실패시킵니다.

새 내부 `billing.Service`는 정확한 signed int64 금액과 날짜·이름 원문을 보존하고 결제수단·영수증·세금계산서 URL을 제외합니다.
합성 검사는 2^53보다 큰 금액, 잘못된/null/overflow 데이터, 페이지 메타데이터·중복 페이지, 순회 상한, 취소,
잘못된 필터, 개인정보 제외와 부분 결과 없는 오류를 다룹니다. 청구 Terraform 타입은 아직 등록하지 않았습니다.
`TestAccBillingReads`가 Go race에서 18.07초로 통과했으며 정확한 조합·독립 단측·음수 범위 필터도 대조했습니다. [청구 계약](billing-contract.md)을 참고하세요.

목록의 기존 청구 ID 두 개와 문서의 BILL-live 상세 ID는 HTTP 403 CHECK_IP·중첩 code 9를 반환한 반면 같은 로컬 인증정보의
현재/목록 조회는 성공했습니다. 상세 조회는 미확인 상태이며 허용 IP나 계정 설정을 변경하지 않았습니다.
통화 배율을 만들지 않으며 시간대·다른 통화·환불/크레딧·미납 변형·상세 중첩 정보는 추가 근거가 필요합니다.
T042는 진행 중으로 바꿨으며 웹메일 정리 T056과 전체 Goal은 미완료입니다. 청구 기록·클라우드 리소스를 변경하지 않았습니다.

## 청구 Terraform Data Source (T072), 2026-09-19

`iwinv_current_bill`·`iwinv_bills`를 등록했습니다. 현재 예상 금액은 정확히 한 행을 요구하고 모든 필드를 민감 표시하며,
청구 전체 목록과 금액 필터도 민감 값입니다. 값은 state/plan에 남지만 결제수단·영수증/세금계산서 링크는 스키마와 산출물에서 제외합니다.
unknown 필터를 계정 전체 조회로 바꾸지 않습니다. 입력 날짜는 요청 전에 검증하고 관측 날짜 원문·정확한 signed int64 금액은
시간대·통화 변환 없이 보존합니다.

`TestAccBillingDataSources`가 Terraform 1.14.2·Go race에서 23.68초로 통과했습니다. 현재/전체/빈 필터 결과,
state 민감 표시와 안정적인 관측 구간의 무변경 plan을 확인했습니다. 합성 Core는 2^53 초과 금액, plan/state 민감 표시,
루트 출력 거절, 저장 산출물 개인정보 제외, 여러 페이지 안정 정렬, 현재 결과 0개/여러 개, 오류·중간 실패와 unknown·0·음수 필터를 검증합니다.
청구나 클라우드 변경은 수행하지 않았습니다.

개발 지원은 Data Source 14개·관리 리소스 7개입니다. 상세의 CHECK_IP, T042 시간대·상세 공백과 웹메일 정리 T056은 미해결입니다.
예상 금액이 바뀌면 향후 plan도 바뀔 수 있으며 금액을 고정한다고 보장하지 않습니다.
[현재 예상 청구 가이드](../../docs/ko/data-sources/current_bill.md) · [목록 가이드](../../docs/ko/data-sources/bills.md).

## T073: 보안 그룹 Data Source (2026-09-19)

기존 검증된 네트워크 어댑터에 `iwinv_security_groups`와 정확한 ID 조회 `iwinv_security_group`을 등록했습니다. 합성 Core는 51행 페이지 순회·응답 순서 변경·중복 이름·설명 한 번 디코딩·null/빈 설명 구분·없는/복수/다른 ID·권한 및 중간 페이지 오류·API 호출 전 잘못된 ID 거절·apply까지 미루는 unknown 참조를 확인합니다. 인라인 규칙·연결 필드는 제외하며 원격 소유권·import는 없습니다. 다른 사용자가 페이지 사이에 목록을 바꾸는 경우 원자적 스냅샷을 보장하지 않습니다.

실환경 Terraform 1.14.2 + Go race acceptance가 **26.94초**에 통과했습니다. 이번 실행에서 새 연결 없는 그룹 하나를 만들고 전체 목록과 상세의 일치, 한글·HTML 특수문자 설명 왕복, fixture 외부 이름 변경의 refresh 반영과 후속 무변경 plan을 검증했습니다. 읽기 전용 Provider 어댑터는 모든 쓰기를 거절했고 별도 fixture 어댑터만 생성·수정·삭제했습니다. 비공개 0600 journal에 생성 ID를 기록했고 삭제 승인 응답 후 상세와 전체 목록 모두 부재를 확인했습니다. 시작 전 그룹 ID는 모두 계속 보였습니다. 실제 ID·원응답·state·키는 공개하지 않습니다. 실환경 50개 초과 그룹·null 설명·연결·트래픽 효력은 이번에 검증하지 않았습니다.

개발 지원은 Data Source 16개·리소스 7개입니다. 이번 fixture는 정리됐지만 별도 웹메일 정리 실패 T056, Compute 제한, 청구 상세 접근, 나머지 기능과 Registry 릴리스 때문에 전체 완료는 아닙니다.

## T074: 블록 스토리지 타입 (2026-09-19)

타입이 있는 `storage.Service` 어댑터와 `iwinv_block_storage_types`를 등록했습니다. 공식 API는 선택적인 `type` 필터와 GB 범위를 설명합니다. 응답 스키마는 존 배열로 설명하지만 예시에는 null도 있습니다. 실측은 문서의 202 대신 HTTP 200, SSD 10–2000 GB와 null 존, SATA 10–20000 GB와 존 하나였습니다. 예시의 SATA 존은 실측과 다릅니다. SSD/SATA 정확한 필터는 전체 카탈로그의 해당 행과 일치했습니다. 알 수 없는 타입은 HTTP 400 `CHECK_PARAM`으로 거절됐으며 이 오류를 그대로 유지합니다. enum·기본 상품·null 존의 의미를 만들어내지 않습니다.

T074 실환경 Terraform 1.14.2 + Go race가 **15.10초**에 통과했습니다. 전체/SSD/SATA 조회, null 보존, 필터와 전체 행의 일치 및 무변경 plan을 검증했습니다. 별도 인증된 읽기 전용 probe로 지원하지 않는 필터 오류도 확인했습니다. 리소스 생성·변경은 없었고 원응답·Terraform 산출물은 비공개로 보관합니다.

어댑터 단위 및 Core 테스트는 2^53 초과 int64, null/빈 존 목록 구분, 존 코드 원문, 타입/존 순서 변경, 잘못되거나 누락된 필드·메타데이터, 중복 타입/존, 필터 불일치, 정상 빈 결과, API 오류, unknown 필터의 전체 조회 방지를 검증합니다. 존 구분자나 GB 단위를 바꾸지 않고 정렬하며, 페이지 없는 카탈로그의 행 수를 확인합니다. 원격 소유권·import 없는 조회 기능이며 볼륨 생성·연결·resize·삭제·과금·생성 자격은 미검증입니다.

개발 지원은 Data Source 17개·리소스 7개입니다. Compute 생성 제한, 별도 웹메일 정리 실패 T056, 청구 상세 접근과 전체 릴리스 게이트는 계속 열어 둡니다.

출처: [API 타입 조회](https://iwinv.readme.io/reference/getv1blockstoragestypes), [CLI](https://docs.iwinv.kr/developers/cli/commands/block-storages). 기존 CLI v0.2.2 대장의 types 하위 명령에도 `--type` 옵션이 있습니다.

## T075: 웹메일 상품 카탈로그 (2026-09-19)

읽기 전용 타입 어댑터와 `iwinv_webmail_products`를 등록했습니다. 인증된 실제 응답은 SHARE 상품 12개로, 서로 다른 비어 있지 않은 ID의 사용 가능 상품 4개와 빈 ID·서로 다른 이름의 준비 중 상품 8개였습니다. HTTP 200에 count·페이지 메타데이터는 없었습니다. 공식 endpoint 문서에는 응답 필드 스키마나 필터가 없으므로 이 상세 계약은 문서 보장이 아닌 실측으로 구분합니다.

실환경 Terraform 1.14.2 + Go race의 조회·무변경 plan acceptance가 **5.09초**에 통과했습니다. 어댑터/Core 합성 테스트는 빈 ID의 모든 행, 다른 ID의 중복 표시 이름, 한글/HTML 모양 이름 원문, ID/유형/이름 정렬을 보존하고 누락/null/잘못된 필드·중복 ID/모호한 준비 중 행·메타데이터 변경·오류를 부분 결과 없이 거절합니다. 취소된 context는 요청하지 않으며 정상 빈 배열은 빈 목록입니다. enum이나 자동 상품 선택은 도입하지 않습니다.

상품 ID·이름·상태·유형만 노출합니다. 가격·디스크·트래픽 단위는 별도 계약을 확보하기 전까지 제외합니다. 클라우드 리소스 생성·메일 발송·DNS 변경은 없었고 인증된 원응답과 테스트 산출물은 비공개입니다. 웹메일 서비스/계정 Read·삭제·import 문제를 해결한 것은 아닙니다. T040은 진행 중이고 별도로 남은 테스트 서비스 때문에 정리 T056은 실패 상태입니다. 개발 지원은 Data Source 18개·리소스 7개이며 나머지 API 기능과 Registry 게시도 미완료입니다.

출처: [공식 웹메일 상품 조회](https://iwinv-webmail.readme.io/reference/웹-메일-상품-조회).

## MCP 인증과 캡처 준비 (2026-09-19)

C27/T014는 미완료입니다. [MCP 조사 문서](mcp-audit.md)에 실제 401 scope challenge, 공개 OAuth/S256 발견, 임시 클라이언트
등록 한 번(201)을 기록했습니다. 소유자 동의는 하지 않았고 SDK 세션은 토큰·인증된 도구 목록 없이 종료됐습니다. 반환된 관리 URL의
DELETE는 404였지만 GET은 같은 신규 등록과 200을 반환했고 관리 자격증명도 변하지 않아 정리는 미확인입니다. 추가 클라이언트·
권한 부여·tools/call은 시도하지 않았습니다. 비공개 등록 근거는 대조를 위해 보관하며, 미관측 도구는 0개나 가짜 목록이 아닌 null로 기록합니다.

T076은 cursor 연결·완료·중복·구조 필드·임의 값 제외·오류 값 비노출의 오프라인 합성 검증을 통과했습니다. 문서 CI에서 실행하고
MCP 인증·도구 실행은 하지 않습니다. 이는 준비 단계이며 실환경 도구 지원 증거가 아닙니다. Provider는 Data Source 18개·리소스 7개로
유지하며 MCP 실행 의존성이나 클라우드 기능을 추가하지 않았습니다.
## T077: 서명 없는 패키지 검증 (2026-09-19)

GoReleaser 2.18.2로 CGO를 끄고 로컬 경로를 제거한 snapshot ZIP 7개를 만들었습니다. 대상은 Darwin amd64/arm64, Linux amd64/arm64/ARMv6/s390x, Windows amd64입니다. 검증기는 정확한 파일·내부 항목, CRC, 실행 권한, 프로토콜 6 manifest를 포함한 SHA-256과 Go 대상 메타데이터를 확인했습니다. 합성 테스트 6개는 손상·누락·중복·프로토콜 차이·추가 및 경로 이탈 파일·실행 권한·사용자 키와 설정의 격리를 확인합니다.

macOS ARM64, Go 1.26.1, 실제 Terraform 1.14.2 실행 파일로 네이티브 `-version`의 `0.0.0-dev`를 확인하고 새 filesystem mirror에서 ZIP을 `init`으로 설치했습니다. 실제 프로토콜 스키마 연결 결과 Data Source 18개·리소스 7개의 이름이 구현 대장과 일치했습니다. 처음 tfenv wrapper를 사용한 시도는 HOME 격리로 버전 설정을 읽지 못해 실패했으며 실제 바이너리 경로로 격리를 유지해 통과했습니다. 임시 workspace와 lock 파일은 제거됐고 클라우드 API나 Terraform plan/apply는 실행하지 않았습니다.

읽기 전용 패키지 CI는 비밀키·릴리스 업로드 없이 교차 빌드와 Linux amd64 설치를 반복합니다. 이번 합격은 서명 없는 로컬 설치이며 7개 대상의 실행, GPG 검증, Registry 게시, migration 합격은 아닙니다. T053은 진행 중, T054/T055는 미완료, 정리 T056은 실패 상태입니다. [릴리스 준비](release-readiness.md)에 남은 조건을 기록했으며 Provider 지원 범위는 동일합니다.

## T052: 소개·카탈로그 문서 (2026-09-19)

Provider 소개와 누락된 카탈로그 7종을 한·영으로 추가했습니다. 존, 이미지 목록/상세, 상품 목록/상세, SSH 키 목록/상세입니다. 소개에는 개발 설치, null/빈 값/unknown을 포함한 키별 인증 우선순위, alias의 계정 선택, 조회와 관리의 차이, 미지원 기능과 마스킹한 문제 해결을 설명합니다. 카탈로그 문서는 정확한 ID·출력 타입·정렬·페이지와 오류·소유권/import·기존 실환경 근거의 한계를 다룹니다. 새 API 동작이나 실환경 검증을 추가로 주장하지 않습니다.

등록된 리소스/Data Source 25종 모두 양언어 전용 문서를 갖췄습니다. 오프라인 문서 CI는 구현 대장을 기준으로 해당 페이지와 소개 양쪽을 요구하므로 양언어에서 동시에 누락돼도 파일 집합 일치만으로 통과하지 않습니다. 실제 Terraform 1.14.2 바이너리와 새 Provider 빌드로 추가한 HCL 예제 16개를 인증정보 없이 validate했고, 최상위 속성 표가 런타임 스키마와 일치하며 양언어 HCL이 동일함을 확인했습니다. `scripts/check_intro_docs.py`로 프로토콜 CI의 Terraform 버전별 검사에도 연결했습니다. 최상위 속성 포함 여부를 검사하며 전체 설명의 의미나 모든 중첩 스키마 flag를 자동 증명하지 않습니다.

전체 페이지의 동작·스키마 검토와 Registry 렌더링은 남아 있어 T052는 진행 중입니다. 개발 지원 범위는 Data Source 18개·리소스 7개로 유지합니다. T053의 서명·Registry 설치 및 T056 정리도 미완료입니다.

## 계정 접근·정리 재대조 (2026-09-19 후속 관측)

- C01/C04/C05, T007: 일반 콘솔 로그인 후 기존 서버가 보이는 상태에서 인증된 `GET /v1/instances?fields=3&page_size=100`은 HTTP 200과 빈 목록을 반환했습니다. `GET /v1/zones`는 계속 HTTP 200과 카탈로그 한 행을 반환했습니다. 서버 생성 화면의 일반 존에는 선택지가 있지만 **API** 분류를 선택하면 존 선택지가 표시되지 않았습니다. 생성 폼을 제출하거나 기존 서버를 변경하지 않았습니다. API 카탈로그 노출·성공한 빈 목록만으로 계정 전체 가시성이나 생성 자격을 보장할 수 없음을 재확인했습니다. 서버별·존별 대조와 제한의 원인은 미확인이라 T007은 합격이 아닌 진행 중입니다.
- T056: 작업에서 생성한 웹메일 서비스는 계속 작업중·메일 계정 0개로 표시됐습니다. 콘솔 선택 ID를 비공개 생성 응답·소유권 대장과 해시로 대조했습니다. 삭제 컨트롤 선택 후 삭제 화면으로 이동하거나 삭제 승인 근거를 얻지 못했습니다. 문서에 명시된 `GET /v1/webmail` 재조회는 다시 HTTP 200과 빈 배열이었습니다. 콘솔/API 불일치가 남아 있으며 자원 정리와 과금 종료 모두 확인하지 못했습니다.
- C27/T014: 반환받은 정확한 OAuth 등록 관리 URI를 한 번 조회했으며 HTTP 200과 동일한 임시 client ID를 다시 확인했습니다. 앞선 관리 DELETE는 여전히 HTTP 404 근거만 있습니다. 추가 등록·동의·토큰 발급·tools/call은 실행하지 않았습니다. 등록 정리는 미해결입니다.

선택 대상의 소유권 대조와 인증된 원응답은 mode-0600 비공개 근거로 보관합니다. 공개 문서에는 계정 자원의 ID·주소·이름·OAuth 자격증명을 넣지 않습니다. 세 문제에 대한 구체적인 비공개 문의 초안을 마련했습니다. 공식 사이트는 기술적 문제를 온라인 기술지원으로 접수하도록 안내하며, 작성 폼 전에 기술지원 약관과 작업 지연 특약 동의를 요구합니다. 동의하거나 제출하지 않았으며 외부 발송·약관 동의 단계의 사용자 확인을 요청했습니다. 유료 서비스도 추가 생성하지 않았습니다.

## T078: 일회용 키 서명 검증 (2026-09-19)

GoReleaser 2.18.2로 `0.0.0-dev`의 7개 대상을 빌드하고 유효기간 하루의 임시 RSA-3072 키로 설정된 GPG 체크섬 서명 단계를 실행했습니다. 공개키만 가진 별도 저장소에서 바이너리 분리 서명과 정확한 fingerprint를 확인한 뒤 8개 산출물을 검사했습니다. 체크섬·서명·ZIP 변경, 서명 누락, armor, 예상과 다른 fingerprint, 알 수 없는 공개키의 7가지 실패를 모두 거부했습니다. 검증 후 Darwin arm64의 filesystem mirror 설치와 스키마 handshake도 통과했습니다. GnuPG는 2.5.22이며 임시 키 저장소와 일회용 서명을 제거했습니다. 클라우드 API 요청이나 릴리스·키 업로드는 수행하지 않았습니다.

패키지 CI도 저장된 서명 비밀키나 게시 권한 없이 이 테스트를 반복하도록 연결했습니다. T077의 패키지·빌드·설치 검사는 같은 실행 경로에 유지합니다. T078은 테스트 신원으로 서명 처리만 검증하며 운영 키 보관, namespace 등록, 릴리스 업로드, Registry 설치는 T053의 미완료 항목입니다. T052/T054/T055 및 정리 T056은 그대로입니다. 실행 명령과 한계는 [릴리스 준비](release-readiness.md)에 기록했습니다.

## C33 / T079: 캐시 서비스 API 접근 경로 (2026-09-19)

신규 폐기용 `cache_lite` 서비스 1개가 active 상태에 도달했습니다. 콘솔 상세 페이지가 문서의 이미지 캐시 매니저로 연결됐고 생성한 계정·FTP 비밀번호로 로그인했습니다. 별도 API 키 관리 화면에는 키가 없었으며 발급 시점의 승인 응답을 기다리는 동안 생성하지 않았습니다. 문서의 인증·용량 경로에 미인증 GET을 각각 한 번 호출하자 서비스 고유 응답 구조와 HTTP 400을 반환했습니다. 인증된 데이터 요청, 이미지·폴더 생성, Provider 등록은 수행하지 않았습니다.

생성한 정확한 ID의 API 삭제 성공, 빈 API 목록에서의 부재, 검색 조건 없는 콘솔의 부재를 확인했습니다. 시작 전 캐시 목록도 비어 있었습니다. 기록된 실행에서 삭제 접수·부재를 확인한 캐시 fixture는 이번 1개를 포함해 총 10개이며 웹메일·OAuth 정리 T056은 별도로 미해결입니다. 원응답·실제 ID·생성 로그인 정보는 Git 밖에 유지합니다. [캐시 데이터 API 준비](cache-data-api.md)에 새 연결 근거와 미검증 토큰·전송·소유권·수명주기 계약을 구분했습니다. T079는 진행 중이고 서비스 작업 12개의 지원·실측 플래그는 false를 유지하며 Provider 지원 범위는 Data Source 18개·리소스 7개입니다.

## T080: 전체 Provider 예제와 실행 스키마 참조 (2026-09-19)

소개·카탈로그 16개 예제 검사를 전체 Provider·기능 페이지의 HCL 52개로 확대했습니다. 실제 검증에서 웹호스팅·DBMS·캐시·NAS의 변수 선언과 두 보안 규칙의 부모 그룹 선언 누락을 발견했습니다. 양언어에 이 전제 조건을 넣고 format을 통일했으며 NAS 설명 문자열도 같게 맞췄습니다. 공통 Provider 설정과 빌드한 바이너리로 검사하며 자격증명·init·plan·apply·클라우드 접근은 사용하지 않습니다.

생성된 한·영 참조표는 Provider와 등록된 기능 25개 전체의 중첩 속성, timeout 블록, 스키마 버전, Terraform 타입, 필수·선택·계산·민감·쓰기 전용 및 부모 민감 표시의 상속을 다룹니다. CI는 파일을 다시 쓰지 않고 실제 실행 스키마·구현 대장과 비교하며 각 기능 페이지에서 해당 참조 절로 이동할 수 있습니다. 실행 스키마만으로 기본값·정수 범위·검증기·plan modifier·교체·실제 API 동작은 증명할 수 없어 개별 가이드에서 계속 설명합니다.

임시 복사본에 변수·부모 리소스 누락을 주입하면 실제 validate가 거부해야 하며, 번역본 실행 코드 차이와 중첩 민감 표시 제거도 거부하는 회귀 검사를 추가했습니다. 오류 주입 중 원본 문서는 변경하지 않습니다. 로컬 Terraform 1.14.2가 통과했고 workflow는 1.14.0/1.14.2에서 정상·오류 검사를 실행합니다. T080은 구조·예제 검증이며 전체 설명 검토와 Registry 렌더링이 남아 T052는 진행 중입니다. Provider/API 지원 범위와 정리 T056 상태는 바뀌지 않았습니다.

## T055 workflow 권한 검토 — 2026-09-19

- 전체 workflow와 실제 저장소 Actions 설정을 검토했습니다. 기본 토큰은 읽기 전용, workflow의 PR 승인은 비활성, 첫 fork 기여자는 승인 필요이며 저장소 Actions secret·environment는 없습니다. 전체 SHA Action 고정을 활성화하고 새 API 조회로 반영을 확인했습니다.
- actionlint 1.7.12와 해시를 고정한 zizmor 1.30.1(오프라인 pedantic, low 이상 실패)의 `Workflow audit`를 추가했습니다. 초기 검사에서 패키지 작업의 공유 Go 캐시가 표시되어 복원·저장을 제거했습니다. 서명 검증 산출물의 실행 간 캐시 의존을 없앴으며 설치 전용 GoReleaser 단계가 취약한 산출물을 게시했다고 주장하지 않습니다.
- 로컬 문법·보안 검사와 wheel 해시 검증 격리 설치가 통과했습니다. 임시 합성 workflow에서 제목 템플릿 삽입·잘못된 표현식을 거부했고 write-all 권한은 pedantic 모드에서 발견되어 CI에 명시했습니다. 실패 기준 아래 정보 수준의 작업 표시 이름 누락 5건은 남았습니다. Go race/vet와 문서 검사도 통과했습니다.
- T055를 not_run에서 in_progress로 변경합니다. 실제 외부 fork 실행과 운영 서명·게시 workflow 검토는 미검증이며 운영 키·게시 권한·릴리스는 추가하지 않았습니다. 명령·범위·공개 출처는 [릴리스 준비](release-readiness.md)에 있습니다.

## T052 공식 문서 형식 검증 — 2026-09-19

- 새로 빌드한 Provider 스키마로 HashiCorp tfplugindocs 0.25.0의 `validate`를 실행했습니다. 도구가 이 namespace를 직접 조회하지 못하므로 임시 입력의 정확한 Provider 조회 키만 지원되는 짧은 이름으로 바꾸며 실제 주소·스키마 내용은 유지합니다.
- 도구가 언어 디렉터리를 무시해 한국어 복사본을 별도 임시 docs 루트로 검사했습니다. 양언어 보안 규칙 가이드와 한국어 리소스 문서 4개에서 머리말 누락을 발견해 총 6개를 수정했습니다. 각 28페이지가 공식 형식·등록 기능 페이지 검사를 통과합니다.
- 두 Terraform CI 버전에 같은 검사를 연결했습니다. 영어·한국어 가이드 제목 누락의 실제 도구 오류 주입 검사를 추가했고 기존 네 건과 함께 로컬에서 거부됐습니다. 실행 예제 52개와 전체 스키마 참조도 통과했습니다.
- 호스팅 계열 리소스 4종의 파괴적 교체 설명을 구현과 대조하고, 리소스 블록을 지우면 prevent_destroy도 없어지며 콘솔·API 삭제는 막지 못한다는 내용을 한·영으로 보완했습니다. Provider나 API 동작은 변경하지 않았습니다.
- 공식 브라우저 미리보기에서 56개 본문의 제목·머리말 숨김·예상 표 106개 표시를 확인했습니다. 저장소 전용 상대 링크 91개가 Registry 호스트 아래로 연결되어 명시적 GitHub URL로 수정하고 새 목적지를 확인했습니다. `design/inventory/doc-preview.json`에 페이지별 내용 해시를 기록했으며 로그인·게시는 하지 않았습니다.
- T052는 진행 중입니다. 전체 설명의 의미 일치, 게시된 Registry 메뉴·링크 이동, 실제 게시는 별도 조건입니다. 재현 명령과 도구 범위는 [문서 정책](documentation.md)에 있습니다.


## CLI 범위 대조 수정 — 2026-09-19

기존 47개 도움말 조사에서 루트가 열거한 기본 `help` 하위 항목이 빠져 있었습니다. 기록된 것과 동일한 해시의 v0.2.2 바이너리를 임시 HOME에서 도움말·버전 명령만 재확인했고 해당 항목을 추가해 48개가 됐습니다. 전체 양언어 대응 대장은 청구 ID 상세 인자와 객체 복사의 로컬·원격 모드까지 한계를 명시합니다. CI는 발견된 하위 명령 누락, 대응 분류 누락·중복, 등록되지 않은 구현 참조를 거절합니다.

공식 netstat 문서에서 실행 PC의 호스트 연결 상태 진단임을 확인했습니다. 기존 서버 트래픽 조회 후보 분류를 로컬 도구로 수정했고 원격 하위 기능은 제외하지 않았습니다. 범위 개요의 오래된 지원 수와 호스팅 이름도 Data Source 18개·리소스 7개 및 `iwinv_webhosting`으로 바로잡았습니다. 기능 대장의 불일치를 해소한 것이며 T050과 미검증 API·CLI 동작은 여전히 열려 있습니다. [전체 범위](coverage.md)와 [대응 대장](../inventory/cli-mapping.json)을 참고하세요.


## P2 인스턴스 조회 준비 — 2026-09-19

고정 마스크 목록·정확한 ID 상세를 조회하는 미등록 Compute 어댑터를 추가했습니다. 합성 테스트는 전체 페이지의 한도·타입·식별자, null·빈 값 보존, 안전한 오류 및 요청하지 않은 비밀번호·VNC 필드 제외를 검증합니다. 조건부 실환경 읽기는 API-visible 0건으로 통과했으며 데이터가 있는 조회·상세·부재 의미는 미검증입니다. T008만 진행 중으로 바꾸고 API 지원 플래그와 Data Source 18개·리소스 7개 등록은 유지합니다. [계약과 남은 조건](instance-read-contract.md)을 참고하세요.


## P2 인스턴스 쓰기 준비 — 2026-09-19

한 대 고정 multipart 생성·마스크를 지정한 정보 수정·정확한 ID 삭제 접수를 위한 별도 미등록 쓰기 어댑터를 추가했습니다. 생성 접수 정보의 후속 검증 실패에도 확보한 ID를 보존하고 예상 밖 복수 ID는 첫 결과를 고르지 않고 대조 대상으로 남깁니다. 삭제는 남는 볼륨 정보를 반환하지만 볼륨 삭제·부재 판정·state 제거를 수행하지 않습니다. 합성 및 TLS multipart 쿼리·서명 테스트가 통과했고 새 실환경 쓰기는 실행하지 않았습니다. T025는 진행 중이며 실제 생성 실패 T015와 Core state T029를 합격 처리하지 않습니다. [쓰기 계약](instance-write-contract.md)을 참고하세요.

## API 접근 복구·웹메일 정리 재대조 — 2026-09-23

테스트 API 키의 허용 목록에 현재 출발 IPv4의 단일 호스트 범위를 추가한 뒤, 같은 키의 문서화된 웹메일 목록 조회가 HTTP 200/SUCCESS와 빈 배열을 반환했습니다. 추가 전에는 HTTP 403/CHECK_IP였습니다. 공개 문서에는 키·IP·원응답을 싣지 않습니다.

인증된 콘솔의 통합 History는 앞서 소유권을 기록한 테스트 웹메일 서비스의 삭제를 2026-09-21 10:15:14에 표시했고, 같은 계정의 검색 조건 없는 웹메일 목록도 비어 있습니다. 따라서 앞서 기록한 **웹메일 잔존** 문제는 정리됐습니다. API 빈 목록만으로 삭제를 판단하지 않았으며 정확한 과금 종료 시각은 아직 확인하지 않았습니다. OAuth 임시 등록 정리는 별도로 미확인이고 전체 T056은 여전히 실패 상태입니다. Compute의 API 지원 존·생성 자격, MCP 도구 목록, Registry 출시는 이 재대조로 검증되지 않았습니다. [현재 실행 순서](roadmap.md)를 참고하세요.

## #1 인증·Compute·MCP 읽기 재검증 — 2026-09-23

동일한 테스트 키와 `GET /v1/webmail` 경로에서 출발 IP가 허용 목록에 없을 때 HTTP 403/`CHECK_IP`(`0x9`), 단일 호스트 범위가 등록된 뒤 HTTP 200/`SUCCESS`(`0x00`)를 확인했습니다. 두 결과는 비공개 요청 기록에 남겼고 키·IP·원응답은 공개하지 않았습니다. 이는 [공식 API 키 가이드](https://docs.iwinv.kr/developers/api/api-key-management/)가 설명하는 허용되지 않은 출발 IP의 403 동작과 일치하므로 **T003을 통과**로 판정합니다. 임의의 IP 대역 전체나 IPv6 처리까지 검증했다는 뜻은 아닙니다.

이어 안전한 읽기 요청으로 `GET /v1/zones`는 HTTP 200/`SUCCESS`와 존 1개, `GET /v1/instances?fields=3599&page_size=100`은 HTTP 200/`SUCCESS`와 빈 목록을 재확인했습니다. 이것은 존의 **카탈로그 노출**만 입증하며 생성 자격이나 콘솔 기존 서버의 API 관리 가능성을 입증하지 않습니다. 앞선 생성 제한 오류를 해결할 근거가 없어 POST를 재시도하지 않았습니다. 비공개로 보존한 임시 MCP 클라이언트의 정확한 등록 관리 URI도 인증된 GET에서 HTTP 200과 동일한 등록을 반환했습니다. DELETE 404 이후 등록 정리는 계속 미확인입니다. 공급사에 Compute 이용 조건과 등록 정리 절차를 묻는 초안을 최신 상태로 고쳤으며, 해결된 웹메일 삭제 요청은 제외했습니다.
