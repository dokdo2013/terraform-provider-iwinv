# P1 API 계약 검증 진행 현황

[English](../en/contract-progress.md) · [검증 계획](verification.md)

관측일: 2026-09-19 UTC. [이슈 #1](https://github.com/dokdo2013/terraform-provider-iwinv/issues/1)의 중간 구현 기록이며,
Provider 제공 또는 P1 완료를 의미하지 않습니다. 현재 클라이언트는 단일 GET 요청을 지원합니다.

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
T011의 클라이언트 격리는 모의 테스트 근거입니다. redirect/오류 비밀정보 차단 테스트는
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

모듈 최소 Go 버전은 1.25.8, CI는 1.25.8과 1.26.1로 구성합니다. 초기 클라이언트는 외부 의존성이 없습니다.
로컬 계약 테스트는 Go 1.26.1, macOS arm64에서 실행했습니다. Terraform 1.14.2가 설치되어 있으나
Provider CLI acceptance는 아직 실행하지 않았습니다.

P2는 확인한 [공식 scaffolding 의존성 기준](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/go.mod)에
맞춰 Framework 1.19.0과 plugin-testing 1.16.0을 선택합니다. Provider 도입 시 버전을 고정하고
컴파일할 예정이며 현재 검증된 의존성으로 표현하지 않습니다.
초기 Provider의 호환 목표는 Terraform >=1.14로 정하고 출시 전 최소 버전을 별도 검증합니다.
[Ephemeral Resource](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources)는 >=1.10,
[write-only 인자](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments)는 >=1.11이 필요하지만
개별 기능의 하한이 이 프로젝트의 목표 하한을 낮추지는 않습니다.
Action은 공개 전에 별도 acceptance와 state 정합성 검증을 통과해야 합니다.
T013은 Provider/프로토콜 버전 조합 검증 전까지 진행 중입니다.

출처: [iwinv 서명](https://iwinv-common.readme.io/reference/api-request),
[응답](https://iwinv-common.readme.io/reference/api-response),
[존](https://iwinv.readme.io/reference/getv1zones),
[상품](https://iwinv.readme.io/reference/getv1flavors),
[이미지](https://iwinv.readme.io/reference/getv1images).
