# 전체 기능 범위

[English](../en/coverage.md) · [목차](../../README.md) · 리비전: 2

목표는 공식 iwinv API·CLI의 모든 기능을 대장으로 관리하고 제어 가능한 원격 리소스 전체를 지원하는 것입니다.
모든 CLI 명령을 영구 리소스로 만들지는 않습니다. 관리 리소스(R), 조회(D), 일회성 작업(A),
임시 값(E), 로컬 도구(L), 조사/계약 공백(G)으로 분류합니다.
**현재 구현·실환경 검증된 Data Source는 `iwinv_availability_zones`, `iwinv_images`, `iwinv_image`, `iwinv_instance_types`, `iwinv_instance_type`, `iwinv_ssh_keys`, `iwinv_ssh_key` 7개입니다. `iwinv_security_group`의 그룹 속성도 구현·실환경 검증했으며, 규칙·연결과 나머지 관리 리소스는 제안 단계입니다.**
실행 방법은 [개발 가이드](development.md), 추적 정보는 [구현 대장](../inventory/implementation.json)에 있습니다.

## Control-plane 기능 대장

[기계 판독 대장](../inventory/api.json)에 각 메서드/경로, 입력 필드, 전송 형식, 응답 코드, 출처가 있습니다.
조사 결과는 IaaS 36 + 공통 4 + 호스팅 5 + 캐시 5 + DBMS 5 + NAS 5 + 웹메일 6 = 66개 HTTP 작업,
전체 문서 75페이지입니다.

| 기능 | Terraform 설계안 | 분류 | 확정 전 과제 |
| --- | --- | --- | --- |
| 존 | `iwinv_availability_zones` | D | 존 상태·사용 조건·상품 호환성 |
| 사양 | `iwinv_instance_type(s)` | D | ID·필터·단위 |
| 이미지 | `iwinv_image(s)` | D | 공개/비공개 형태, 단일 결과 명확성 |
| 서버 | `iwinv_instance(s)` 조회와 `iwinv_instance` 리소스 | R/D | C01–C13, C17 |
| 전원/재부팅/재설치 | 독립 Action 후보, power-state 리소스는 ADR 후 결정 | A/G | 중단·재실행·state 정합성·과금 |
| 원격 콘솔 | `iwinv_instance_console` 후보 | E | 만료·노출 정책; 일반 state에 URL 저장 금지 |
| User Script | `iwinv_user_script(s)` 조회, 추후 관리 | D/G | 콘솔 전용 생성, C18 |
| 블록 스토리지 종류 | `iwinv_block_storage_types` | D | 상품별 크기·종류 |
| 블록 스토리지 | `iwinv_block_storage(s)` 조회 및 리소스 | R/D | C14–C15, 생성 시 연결 소유권 |
| 블록 연결 | `iwinv_block_storage_attachment` 후보 | R/G | 볼륨 연결 소유자와 충돌 금지 |
| 보안 그룹 | `iwinv_security_group(s)` 조회 및 리소스 | R/D | Read·ICMP·삭제 제약 |
| 보안 규칙 | `iwinv_security_group_ingress_rule`, `iwinv_security_group_egress_rule` | R/D | C16, 정수/문자 ID, inline 소유권 금지 |
| 그룹 연결 | `iwinv_security_group_attachment` | R/D | 복수 연결·추가/교체 의미 |
| 청구/결제 | `iwinv_bill(s)`, `iwinv_current_bill` | D | 개인정보·통화·부가세·시간대 |
| SSH 키 | `iwinv_ssh_key(s)` 조회, 추후 키 리소스 | D/G | 현재 확인된 API는 조회만 제공 |
| 웹호스팅 | `iwinv_web_hosting`, 상품/서버 조회 | R/D | 비밀번호 state 노출·수정 계약 부재·C19–C20 |
| 컨텐츠 캐시 | `iwinv_content_cache`, 상품 조회, referrer 집합 | R/D/G | C19–C22; purge는 별도 서비스 API |
| 클라우드 DBMS | `iwinv_db_instance`, 상품 조회, 허용 IP 집합 | R/D/G | C19–C21; 파괴적 교체·백업 의미 |
| API NAS | `iwinv_shared_storage`, 상품 조회, 허용 IP 집합 | R/D/G | C19–C21; 데이터 보존·프로토콜별 접근 |
| 웹메일/계정 | `iwinv_webmail`, `iwinv_webmail_account`, 상품 조회 | R/D/G | C19–C20, C23; 계정 Read·비밀번호 처리 |

`(s)`는 단일/목록 조회 후보의 축약이며 실제 식별자가 아닙니다.
66개 작업은 모두 위 그룹에 속합니다. 없는 수정/조회 API는 미확정으로 남기며 가상의 경로를 만들지 않습니다.

## 서비스 API와 추가 조사

| 기능과 출처 | 장기 지원 방향 | 상태 |
| --- | --- | --- |
| [오브젝트 API](https://help.iwinv.kr/manual/736)와 공식 CLI | 버킷/객체 조회, 객체 및 검증된 버킷 설정(R/D), copy/move(A), presign(E) | endpoint·서명·multipart·페이지·ACL/versioning/lifecycle 호환표 필요; CLI 목록만으로 버킷 생성 가능하다고 단정 금지 |
| [NAS 서비스 API](https://help.iwinv.kr/manual/763) | 서비스 내부 작업과 가입/해지 수명주기 구분 | G: 작업·인증을 열거하고 R/D/A에 매핑 |
| [캐시 서비스 API](https://help.iwinv.kr/manual/938) | 설정/조회와 purge | G: 별도 계약 조사, 가입 API만으로 범위 완료 처리 금지 |
| [문자](https://docs.iwinv.kr/developers/api/Message_api/) | SMS/LMS/MMS/국제 발송(A), 이력/잔액(D), 지원되는 예약 작업 | URL 수집 완료; POST 조회 구분, quota·예약 시간대·바이트·개인정보 처리 검증 |
| [알림톡](https://docs.iwinv.kr/developers/api/kakao_api/) | 템플릿(R/D), 발송/취소(A), 이력/잔액(D) | 검수 상태별 제약·중복 발송 방지 검증 |
| [MCP](https://docs.iwinv.kr/developers/mcp/) | 인증 후 도구와 기능 대장 비교 | G: tools/list 미확인, API와 동등하다고 가정하지 않음 |
| SDK/기타 상품 | 중앙 개발자 목록 외 API와 신규 서비스 발견 | G: 공개 목록만으로 전체 조사 완료 판단 금지 |

모든 원격 서비스 기능은 장기 조사 범위에 포함합니다. 발송·객체 이동 같은 데이터 작업은 명시적 실행 의미를 정의하고,
refresh나 일반 apply에서 예상 밖으로 실행하지 않습니다. 임의 HTTP 요청 기능으로 전체 지원을 대신하지 않습니다.

## CLI 분류

[CLI 근거 대장](../inventory/surfaces.json)은 공개 명령 참조이며 설치된 바이너리 검증 결과가 아닙니다.

| CLI 그룹 | Provider 대응 |
| --- | --- |
| instances, block-storages, flavors, images, zones, ssh-keys, user-script | 위 R/D/A/E 그룹에 연결 |
| bill, netstat | 청구/트래픽 조회, netstat의 공식 API 대응 조사 |
| object-storage auth/ls/ll/cp/mv/rm/presign | 인증은 설정, 목록은 D, 객체 수명주기는 R, 작업은 A, presign은 E |
| account, login, logout | 로컬 인증/프로필(L), Provider는 설정·환경변수·alias 사용; 비밀 출력 금지 |
| completion, help, theme, install/reinstall/update/uninstall | 로컬 도구 동작(L), 클라우드 지원 누락으로 세지 않음 |
| 비공개 admin·알 수 없는 하위 명령 | G: 지원되는 계약 확보 전 help 이름만으로 구현 금지 |

## 지원률 계산

발견·분류·계약 검증·구현·acceptance 통과·출시를 별도로 기록합니다.
지원률 분자는 출시와 테스트가 끝난 기능만 포함하며 문서 다운로드 수를 사용하지 않습니다.
G 항목은 분모와 공백 대장에 남기고 L 제외 항목은 별도 보고합니다.
새 서비스마다 조사·수명주기/소유권 ADR·인증·오류·검증·양언어 문서가 필요합니다.
대장은 수동 갱신 후 diff를 검토하고 지원 상태를 변경합니다.
