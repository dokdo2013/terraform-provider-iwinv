# Terraform Provider for iwinv

[English](README.en.md) · 한국어

AWS Provider에 익숙한 Terraform 사용자를 위한 독립적인 iwinv 커뮤니티 Provider 프로젝트입니다.

**현재 상태: Data Source 16개와 보안 그룹·ingress·egress·웹호스팅·DBMS·캐시·NAS 리소스 7개를 구현하고 실환경 검증했습니다. Registry 릴리스는 아직 없습니다.**
실행 가능한 범위는 [개발용 실행 안내](design/ko/development.md)와 [보안 그룹 가이드](docs/ko/resources/security_group.md)를 참고하세요. 인스턴스·연결 등 나머지 설계 예제는 아직 적용할 수 없습니다.
스마일서브/iwinv의 공식 제품 또는 공식 지원 프로젝트가 아닙니다.

## 목표

- 공식 API·CLI가 제공하는 리소스와 작업 전체를 기능 대장으로 추적하고 단계적으로 지원합니다.
- 생성·조회·수정·삭제, 기존 인프라 import, 변경 감지, 예측 가능한 plan을 제공합니다.
- AWS 스타일의 사용 패턴을 제공하되 iwinv에 없는 기능은 만들지 않습니다.
- 한국어와 영어 문서, 예제, 문제 해결 가이드를 함께 유지합니다.

## 설계 읽기

| 문서 | 내용 |
| --- | --- |
| [사용자 경험 및 아키텍처](design/ko/architecture.md) | AWS 스타일 매핑, 스키마, 상태·인증·오류 설계 |
| [호스팅 수명주기 설계](design/ko/webhosting-lifecycle.md) | 24시간 계정명 재사용 제한, 비밀번호·import·교체 정책; SHARE PHP 8.4 실환경 검증 |
| [캐시 수명주기 설계](design/ko/cache-lifecycle.md) | 중첩 비밀번호·리퍼러 전체 교체·작업중 오류·정리 계약; Terraform import·교체·비밀번호 비저장 검증 |
| [NAS 수명주기 설계](design/ko/nas-lifecycle.md) | IP별 RO/RW 전체 맵, 공유 이름 이력 없는 import, 용량 교체·정리 |
| [DBMS 수명주기 설계](design/ko/dbms-lifecycle.md) | 전체 허용 IP 집합, 생성 계정 이력·상품 모호성·import·복구 |
| [청구 조회 계약](design/ko/billing-contract.md) | 금액·날짜·페이지·민감 plan/state 처리; 예상 청구·목록 조회 지원, 상세 권한 제한 |
| [API 계약 및 제약](design/ko/api-contract.md) | 확인된 사실, 문서 불일치, 실제 검증이 필요한 사항 |
| [전체 기능 범위](design/ko/coverage.md) | API·CLI·서비스별 Resource/Data/Action/Ephemeral 분류 |
| [검증 계획](design/ko/verification.md) | 단계별 합격 기준과 실행 체크리스트 |
| [개발 로드맵](design/ko/roadmap.md) | 의존 관계와 작업별 완료 조건 |
| [한국어·영어 문서 정책](design/ko/documentation.md) | 초보자 가이드, 용어, 번역 동기화 |
| [증거와 출처](design/sources.md) | 조사일과 공식 문서 링크 |

## 진행 상태

- [x] 공식 문서 기반 1차 조사 및 설계 저장소 구성
- [x] 공개 control-plane 문서 75페이지에서 66개 HTTP 작업 식별
- [x] CLI 및 메시징 문서 21페이지의 명령·URL 참조 정리
- [ ] 실제 계정의 API 응답 및 동작 검증
- [x] Go 조회·쓰기 클라이언트 및 합성 계약 테스트
- [x] Provider 골격과 존·이미지·상품·SSH 키 Data Source의 실환경 읽기 acceptance
- [x] 보안 그룹 속성 리소스의 create/update/import/drift/destroy 검증
- [x] ingress/egress 규칙의 import·drift·교체·의존성 삭제 검증
- [x] 웹호스팅 상품·서버 카탈로그 조회와 무변경 plan 검증
- [x] 웹호스팅의 새 계정 교체/import/비밀번호 비저장/삭제 검증 (SHARE PHP 8.4)
- [x] DBMS 상품의 전체/엔진/등급 필터 조회와 빈·버전 간 중복 ID 보존 검증
- [x] DBMS 허용 IP 전체 교체·import·drift·새 계정 교체·정리 검증 (STD Redis)
- [x] 캐시 리퍼러 수정·새 계정 초기화 교체·import·비밀번호 비저장·정리 검증 (`cache_lite`)
- [x] 캐시 상품 전체/SHARE/SINGLE 조회와 null ID 보존·무변경 plan 검증
- [x] [NAS 리소스](docs/ko/resources/shared_storage.md)의 active 대기·권한 수정·import·용량 교체·정리 검증
- [x] [NAS 상품 카탈로그](docs/ko/data-sources/shared_storage_products.md)의 빈 ID·null 버전·GB 범위 보존과 무변경 plan 검증
- [x] [현재 예상 청구](docs/ko/data-sources/current_bill.md)·[청구 목록](docs/ko/data-sources/bills.md)의 정확한 금액·민감 plan/state·실환경 조회 검증
- [ ] 나머지 관리 리소스 구현
- [ ] 리소스별 acceptance test 및 서명 릴리스
- [ ] Terraform Registry 게시

위 숫자는 문서 조사 범위이며 구현 지원률이 아닙니다. 서비스별 API, S3 호환 기능,
CLI v0.2.2 하위 명령·옵션도 조사했으며 인증 후 MCP 도구는 미확인입니다. [구현 대장](design/inventory/implementation.json)에 검증된 기능만 별도로 기록합니다.

진행 근거와 실행 방법: [P1 API 계약 검증 현황](design/ko/contract-progress.md).

## 기여 및 검증

[기여 안내](CONTRIBUTING.md)를 먼저 읽어주세요. 현재 문서 검증은 Python 3.10 이상으로 실행합니다.

```sh
python3 scripts/check_docs.py
```

공개 문서를 다시 조사하려면 다음 명령을 수동 실행하고 차이를 검토합니다. 계정 인증이나 리소스 생성은 하지 않습니다.

```sh
python3 scripts/discover_api.py
python3 scripts/discover_surfaces.py
```

라이선스: [MPL-2.0](LICENSE). 회사의 기존 코드·설정·계정 데이터 없이 작성한 독립 프로젝트입니다.
