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

T001, T002, T003, T004, T006, T009의 일부 근거를 확보했습니다.
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
