# 검증 계획

[English](../en/verification.md) · [목차](../../README.md) · 리비전: 1

**Data Source 18개의 인증된 읽기와 보안 그룹·규칙·호스팅·DBMS·캐시·NAS 리소스 7개의 수명주기 acceptance를 통과했습니다. 서버와 나머지 관리 리소스 검증은 미완료입니다.**
[현재 근거](contract-progress.md)에서 실측과 모의 테스트를 구분합니다. 문서 CI는 저장소 정합성만 검사합니다. 한·영 [체크리스트](../inventory/checks.json)가 공통 작업 대장입니다.
각 항목에는 고유 ID, 검증 방법, 양언어 합격 조건, 단계와 실행 상태가 있습니다. `in_progress`는 일부 근거만 확보된 상태로 합격이 아닙니다.

## 증거 및 실행 규칙

검증 ID, commit, Provider/Terraform/Go 버전, 환경 종류, API/상품/존, UTC 시각,
입력 fixture 참조, 마스킹 결과, 성공/실패, 정리 결과를 기록합니다.
mock 테스트와 실제 서비스 관측을 구분하며 실패·건너뛴 검증을 성공으로 표시하지 않습니다.
실환경 테스트는 범위가 정해진 폐기 가능한 환경·비용 한도·공유 quota에 맞춘 직렬 실행·실행별 생성 ID 대장이 필요합니다.
이름 prefix만으로 임의 리소스를 일괄 삭제하지 않습니다. 신뢰하지 않는 fork PR에서는 변경 테스트나 메시지 발송을 실행하지 않습니다.
계정·출발 IP·예산·정리 절차가 준비되기 전에는 과금 가능한 acceptance workflow를 활성화하지 않습니다.

## 검증 순서

1. **G0 문서:** API/CLI 공백 열거, 모든 확인된 기능 분류, 한·영 문서 동기화.
2. **G1 조회 계약:** 인증·전체 페이지·ID 매핑·타입·기존 서버 노출 검증.
3. **G2 수명주기:** 서버 한 대의 import·무변경 plan·외부 변경 감지·삭제 완료 검증.
4. **G3 실패 복구:** 요청 제한·timeout·생성 결과 불명확·취소·부분 state·정리 검증.
5. **G4 서비스 확장:** 각 서비스에 수명주기/import/drift/실패 테스트 반복, 소유권 충돌 해소.
6. **G5 출시:** 빌드된 Provider로 예제·문서·migration·호환 버전·서명·새 환경 설치 검증.

서비스별로 검증을 통과하면 구현을 진행하되 미해결 기능은 지원되지 않음을 표시합니다.
추가 서비스 API/CLI/MCP의 조사 공백이 남아 있으면 전체 기능 지원 완료라고 선언하지 않습니다.

## 리소스별 필수 시나리오

객체 한 개 생성 → 완료 대기 → refresh → 무변경 plan → 수정 가능한 필드 변경 → 무변경 plan →
별도 테스트 state로 import → 지원 속성 비교 → 실제 값에 맞는 설정으로 무변경 plan →
Terraform 외부에서 변경 → drift 감지 → 복원 → 외부 삭제 → 확정된 부재 감지.
destroy·의존성 순서 테스트는 별도로 소유한 fixture를 사용하며 두 state가 동시에 하나의 객체를 수정하지 않게 합니다.
교체 속성은 plan에 교체가 드러나야 하며 지원하지 않는 전이는 변경 전에 오류로 끝나야 합니다.

CLI 기반 acceptance는 `terraform-plugin-testing`, 프로토콜·오류는 HTTP 테스트 서버,
변환·서명은 table-driven 테스트를 사용합니다. 공유 클라이언트·limiter에는 race 검증도 수행합니다.
`ImportStateVerifyIgnore`는 Cxx 근거와 함께 좁게 사용하며 import 결함을 가리는 용도로 쓰지 않습니다.
문서 검사와 일반 단위 테스트에는 인증키가 필요하지 않습니다.

## 완료 증거

출시되는 각 Resource/Data/Action/Ephemeral에 기능 항목·통과한 검증 ID·양언어 문서·불변 릴리스·남은 한계를 연결합니다.
읽기 전용 조사만으로 acceptance를 통과했다고 하지 않습니다.
정리 실패는 대상 ID와 증거를 보존해 보고하며 CI를 성공시키려고 숨기지 않습니다.
작업 순서는 [로드맵](roadmap.md)을 따릅니다. 근거: [acceptance testing](https://developer.hashicorp.com/terraform/plugin/testing/acceptance-tests),
[import](https://developer.hashicorp.com/terraform/plugin/framework/resources/import),
[Read](https://developer.hashicorp.com/terraform/plugin/framework/resources/read).

## 검증 항목

| ID | 단계 | 방법 | 합격 조건 | 상태 |
| --- | --- | --- | --- | --- |
| T001 | P1 | mock/live-read | Timestamp+path 서명과 query/끝 슬래시 처리가 정확하다 | in_progress |
| T002 | P1 | mock/live-read | 시계 오차를 구분하고 재시도마다 새 Timestamp로 서명한다 | in_progress |
| T003 | P1 | live-read | 허용/차단 출발 IP의 인증 결과를 확인한다 | passed |
| T004 | P1 | mock/live-read | HTTP와 업무 오류를 함께 판정하고 알 수 없는 코드를 숨기지 않는다 | in_progress |
| T005 | P1 | mock/live-read | 작업별 JSON/form/multipart 인코딩을 확인한다 | in_progress |
| T006 | P1 | mock/live-read | 모든 ID를 중복 없이 조회하고 올바른 조건에서 페이지를 종료한다 | in_progress |
| T007 | P1 | live-read | 계정 목록과 지원 존의 서버 노출이 콘솔 근거와 일치한다 | in_progress |
| T008 | P1 | mock/live-read | 상세 필드 마스크가 필요한 값만 가져오고 비밀번호/콘솔 토큰을 제외한다 | in_progress |
| T009 | P1 | mock/live-read | 누락/null/빈 값과 중첩 배열/객체를 정확히 구분한다 | in_progress |
| T010 | P1 | mock/live-read | 단일 조회가 0개/복수 결과를 거부하고 첫 원소를 임의 선택하지 않는다 | in_progress |
| T011 | P1 | mock/live-read | alias 간 인증/endpoint/캐시가 섞이지 않는다 | passed |
| T012 | P1 | mock | 인증정보가 다른 호스트 redirect나 진단에 유출되지 않는다 | passed |
| T013 | P1 | review | Go/Terraform/Framework 조합과 기능별 최소 버전을 ADR로 정의한다 | passed |
| T014 | P1 | review | 나머지 서비스 API/CLI 옵션/인증 MCP 도구를 대조하고 공백을 기록한다 | in_progress |
| T015 | P2 | live | 생성 ID 한 개가 안정적이며 후속 실패에도 state에서 보존된다 | failed |
| T016 | P2 | mock/live | 모든 상태와 기한/취소를 완료 대기가 처리한다 | not_run |
| T017 | P2 | live | apply 후 반복 refresh/plan에서 불필요한 변경이 없다 | not_run |
| T018 | P2 | live | 이름/설명 변경/초기화가 교체나 무한 차이 없이 반영된다 | not_run |
| T019 | P2 | live | 한글 NFC/NFD/멀티바이트 길이/공백/특수문자가 정확히 왕복한다 | not_run |
| T020 | P2 | live | import가 지원 속성을 복원하고 일치하는 설정에서 plan이 비어 있다 | not_run |
| T021 | P2 | mock/live | 읽지 못하는 SSH/스크립트 이력에 가짜 기본값을 넣지 않고 import 제약을 검증한다 | not_run |
| T022 | P2 | live | 외부 변경을 감지하고 확정된 삭제에만 state를 제거한다 | not_run |
| T023 | P2 | mock/live | 인증/제한/서버 오류와 잘못된 응답/빈 페이지를 삭제로 오인하지 않는다 | not_run |
| T024 | P2 | mock/live | 생성 직후 404를 제한된 반영 지연 범위에서 처리한다 | not_run |
| T025 | P2 | mock/live | 결과 불명확 생성 요청을 자동 재전송하거나 이름으로 자동 채택하지 않는다 | in_progress |
| T026 | P2 | mock/live | backoff/jitter/Retry-After와 합산 요청 부하가 제한된다 | not_run |
| T027 | P2 | live | resize/교체 plan과 실제 중단/IP/디스크 영향이 일치한다 | not_run |
| T028 | P2 | live | destroy가 실제 부재를 확인하고 잔여 리소스/과금 상태를 구분한다 | not_run |
| T029 | P2 | mock/live | 부분 실패/중단/재실행에서 ID 보존과 문서화된 복구가 가능하다 | not_run |
| T030 | P2 | mock | 병렬 리소스/클라이언트가 race 및 state 오염 검증을 통과한다 | not_run |
| T031 | P3 | live | 보안 규칙 방향/프로토콜/포트/CIDR/ICMP/ID 정규화를 확인한다 | in_progress |
| T032 | P3 | live | 규칙 import/중복/외부 변경/부모 삭제 결과가 결정적이다 | in_progress |
| T033 | P3 | live | 보안 그룹 연결 개수와 추가/교체 의미를 검증한다 | not_run |
| T034 | P3 | live | 스토리지 생성 연결/분리/재연결/존/보존을 검증한다 | not_run |
| T035 | P3 | live | 연결 소유자는 하나이며 서버 삭제 시 관리 데이터 손실을 숨기지 않는다 | not_run |
| T036 | P3 | live | 연결 해제와 볼륨/그룹/서버 삭제 순서를 검증한다 | not_run |
| T037 | P4 | live-read | 추가 서비스마다 응답 타입/ID/오류/전체 목록 계약을 확보한다 | in_progress |
| T038 | P4 | live | 서비스별 수명주기/import/무변경 plan/drift를 출시 전에 검증한다 | not_run |
| T039 | P4 | live | 허용 IP/referrer의 추가/교체/빈 집합/외부 변경 동작을 검증한다 | in_progress |
| T040 | P4 | live-read | 메일 계정을 관리하기 전에 신뢰 가능한 조회를 확보한다 | in_progress |
| T041 | P4 | mock/live | 비밀번호 로그 노출을 막고 state/write-only 동작을 명시한다 | in_progress |
| T042 | P4 | live-read | 청구 단위/통화/부가세/시간대/민감 필드를 정확히 문서화한다 | in_progress |
| T043 | P5 | mock/live | 오브젝트 서명/주소 형식/페이지/지원 기능 호환성을 검증한다 | not_run |
| T044 | P5 | live | 객체 hash/ETag/multipart/version/삭제 의미를 검증하고 미지원 설정을 강제하지 않는다 | not_run |
| T045 | P5 | mock/live | 임시 콘솔/presign 값이 plan/state에 남지 않고 만료를 검증한다 | not_run |
| T046 | P5 | mock/live | Action은 명시적으로 실행되며 refresh/결과 불명확 오류에서 재실행되지 않는다 | not_run |
| T047 | P5 | mock/live | rebuild/Action 결과가 이후 Read와 설정에 정합적이다 | not_run |
| T048 | P5 | live | 메시징 템플릿 검수 상태와 허용 수정/삭제를 검증한다 | not_run |
| T049 | P5 | mock/live | 발송/취소/시간대/바이트/중복 방지를 한정된 테스트에서 검증한다 | not_run |
| T050 | P5 | review | 모든 잔여 원격 기능에 검증된 지원 또는 명시적인 공급사 제약이 있다 | not_run |
| T051 | P6 | CI | 모든 예제가 실제 빌드 Provider로 format/validate를 통과한다 | passed |
| T052 | P6 | CI/review | 한영 문서가 동일한 동작과 계약/검증 ID를 다룬다 | in_progress |
| T053 | P6 | CI | 지원 OS/아키텍처 바이너리/체크섬/서명/새 Registry 설치를 검증한다 | in_progress |
| T054 | P6 | CI/live | 이전 버전 state migration과 의존성 갱신 후 plan이 안정적이다 | not_run |
| T055 | P6 | CI/review | 외부 PR에 실키가 없고 릴리스 권한/Action이 제한된다 | in_progress |
| T056 | P6 | live | 실행별 ID 대장으로 정리를 증명하며 누수는 복구 근거와 함께 실패 처리한다 | failed |
| T057 | P3 | mock/live | 미연결 전용 그룹 속성의 생성/수정/import/무변경 plan/drift/재생성/삭제가 통과하고 생성 실패 ID로 정리할 수 있다 | passed |
| T058 | P3 | mock/live | 소유한 TCP/UDP 규칙의 CRUD/import/무변경 plan/방향 drift/초기화·부모 교체/외부 삭제와 자식 우선 destroy가 통과한다 | passed |
| T059 | P4 | mock/live | 호스팅 어댑터의 카탈로그·두 서비스 생성/조회/삭제·다른 서비스 보존·정확한 ID 정리가 통과하며 Terraform 수명주기와 과금은 별도 검증한다 | passed |
| T060 | P4 | mock/live | 호스팅 Core 수명주기의 새 계정 교체, 생성 이력 없는 import/무변경 plan, ephemeral write-only 값의 plan/state 비저장, 생성 실패 ID 복구와 정확한 ID 정리를 검증한다 | passed |
| T061 | P4 | mock/live-read | 호스팅 카탈로그 Data Source의 SHARE/SINGLE 필터, 정확한 서버 ID, 정렬된 전체 결과, 잘못된 입력/빈 목록/오류와 실환경 읽기/무변경 plan을 검증한다 | passed |
| T062 | P4 | mock/live | DBMS 어댑터가 상품 모호성과 정확한 ID를 보존하고 두 서비스 생성/조회/허용 목록 교체, 다른 서비스 보존과 삭제 응답/정확한 ID 정리를 검증한다 | passed |
| T063 | P4 | mock/live | STD Redis Core 수명주기의 허용 목록 수정/drift, 계정 이력 없는 import/무변경 plan, 새 계정 교체, 실패 시 ID 보존과 정확한 ID 정리를 검증한다 | passed |
| T064 | P4 | mock/live-read | DBMS 상품 Data Source가 빈/중복 ID를 보존하고 전체 응답·필터를 검증하며 모든 공식 필터의 실환경 읽기와 무변경 plan을 통과하되 버전 선택을 약속하지 않는다 | passed |
| T065 | P4 | mock/live | 캐시 어댑터가 정확한 ID와 null 상품을 보존하고 중첩 비밀번호 생성·전체 리퍼러 교체를 검증하며 정확한 작업중 거절만 재조회하고 다른 서비스 보존·삭제 접수·정리를 증명한다 | passed |
| T066 | P4 | mock/live | 캐시 Core의 전체 집합 수정·drift, 빈 집합·새 계정 교체, 비밀번호 없는 import·무변경 plan, ephemeral 비저장, 정확한 작업중 복구와 불확실한 쓰기 재전송 없는 ID 정리를 검증한다 | passed |
| T067 | P4 | mock/live-read | 캐시 상품 Data Source가 null·빈 ID를 보존하고 전체 필터 응답·안정 정렬을 검증하며 전체/SHARE/SINGLE 실환경 조회와 무변경 plan을 통과한다 | passed |
| T068 | P4 | mock/live | NAS 어댑터가 정확한 ID와 공유 이름 이력의 불확실성을 보존하고 active 대기·전체 RO/RW 맵 및 기존 호스트 권한 교체·다른 서비스 보존·소유 ID 삭제 접수와 정리를 검증한다 | passed |
| T069 | P4 | mock/live | NAS Core의 전체 권한 맵 수정·drift, 공유 이름 이력 없는 import·무변경 plan·수정, 새 공유로 용량 교체, 실패 시 ID 보존과 삭제 접수·정확한 ID 정리를 검증한다 | passed |
| T070 | P4 | mock/live-read | NAS 상품 Data Source가 빈 ID·null 버전·정수 GB 범위를 보존하고 잘못된 전체 카탈로그를 거절하며 안정 정렬·실환경 조회·무변경 plan을 통과하고 상품을 자동 선택하지 않는다 | passed |
| T071 | P4 | mock/live-read | 청구 조회 어댑터가 정확한 정수 금액을 보존하고 결제수단·문서 URL을 제외하며 전체 페이지·필터·빈 결과 계약을 검증하고 권한·중간 페이지 오류를 부분 결과로 바꾸지 않는다 | passed |
| T072 | P4 | mock/live-read | 청구 Data Source가 정확한 금액·plan/state 민감 표시·결제정보 제외를 보존하고 표시 없는 출력·unknown/잘못된 필터를 거절하며 읽기 전용 실환경 조회와 안정 구간 무변경 plan을 통과한다 | passed |
| T073 | P3 | mock/live | 보안 그룹 Data Source가 정확한 ID·null 설명·정렬된 전체 페이지를 보존하고 모호한 결과·오류를 거절하며 소유 테스트 그룹의 목록·상세·refresh·무변경 plan과 삭제 확인을 통과한다 | passed |
| T074 | P3 | mock/live-read | 블록 스토리지 타입 Data Source가 nullable 존 목록·정확한 정수 GB 범위를 보존하고 필터·전체 응답을 검증하며 API 오류를 거절하고 실환경 전체/SSD/SATA 조회·무변경 plan을 통과한다 | passed |
| T075 | P4 | mock/live-read | 웹메일 상품 카탈로그가 빈 ID와 서로 다른 준비 중 행을 보존하고 메타데이터를 정렬하며 잘못된/중복/오류 응답을 거절하고 서비스 수명주기 지원을 주장하지 않은 채 실환경 전체 조회·무변경 plan을 통과한다 | passed |
| T076 | P1 | offline | 오프라인 MCP 목록 검증이 불완전/순서 변경/반복 페이지와 중복 도구를 거절하고 설명/기본값/예시/meta/cursor를 제외하며 도구 힌트나 구조 조사를 권한·검증된 API 지원으로 취급하지 않는다 | passed |
| T077 | P6 | offline/CI | 서명 없는 snapshot의 정확한 ZIP·체크섬·프로토콜 메타데이터와 네이티브 filesystem mirror 설치의 등록 스키마를 검증하고 서명·Registry 합격은 구분한다 | passed |
| T078 | P6 | offline/CI | 일회용 RSA 키의 snapshot 바이너리 분리 서명·정확한 서명자·전체 패키지 체크섬을 검증하고 7가지 변조·신뢰 실패를 거부하며 게시나 Registry 합격 없이 테스트 키를 제거한다 | passed |
| T079 | P5 | live | 소유한 캐시 fixture로 상품·관리페이지 연결, 별도 API 키·토큰 인증, 이미지·폴더 읽기·쓰기와 정리를 검증하고 control-plane 키나 미지원 상품을 혼동하지 않는다 | in_progress |
| T080 | P6 | offline/CI | 모든 Provider 페이지의 HCL 예제를 빌드 바이너리로 format·validate하고 한영 실행 코드와 전체 중첩 스키마 참조를 대조하며 주입한 문서 오류를 거부한다 | passed |
