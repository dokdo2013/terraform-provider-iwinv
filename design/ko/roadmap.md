# 로드맵과 작업 단위

[English](../en/roadmap.md) · [목차](../../README.md) · 리비전: 1

초기 커밋은 Provider 구현 완료를 의미하지 않습니다. 단계별 검증을 통과하며 진행하고,
첫 릴리스가 작더라도 최종 목표는 API·CLI가 제공하는 전체 원격 리소스 지원입니다.

| 작업 | 선행 조건 | 산출물 | 완료 조건 |
| --- | --- | --- | --- |
| P0 설계 기반 | 없음 | 공개 레포, 양언어 설계, 기능 대장, 체크리스트, 문서 CI | 로컬/원격 검사 성공, 한계 명시 |
| P1 API 계약 검증 | P0 | 마스킹 조회 fixture, 인증/인코딩/페이지/오류 계약, 나머지 기능 조사, 버전 ADR | G1 통과, 공백별 근거와 담당 |
| P2 Compute 최소 수직 구현 | P1 | Go client, Framework Provider, 카탈로그 조회, 서버 수명주기/import | G2·G3 통과, 무변경 plan, 서명 prerelease 후보 |
| P3 네트워크·스토리지 | P2와 C14–C17 해소 | 그룹/규칙/연결, 연결 소유자가 하나인 스토리지 수명주기 | 리소스별 G4, import·의존성 삭제 검증 |
| P4 기타 control-plane | P1과 서비스별 계약 해소 | 호스팅/캐시/DBMS/NAS/웹메일, 상품, 허용 목록, 청구, 키/스크립트 조회 | 기능별 C19–C23 해소, 비밀번호·파괴적 교체 문서화 |
| P5 서비스·작업 전체 범위 | P1 조사와 관련 P2/P3/P4 | 오브젝트, NAS/캐시 서비스 API, 메시징/템플릿, Action/Ephemeral, MCP 대조 | 명시적 실행·비밀 state 검증; 미지원 항목은 구현 또는 공급사 제약으로 추적 |
| P6 안정 릴리스·사용자 교육 | P2–P5 증거 | 버전/state migration, Registry 배포, 완성된 한·영 가이드 | G5 통과, 정확한 출시 범위; 전체 대조 후에만 전체 지원 선언 |

P2 이후 제한된 범위의 prerelease는 가능합니다. 일부 지원을 전체 지원으로 소개하지 않으며,
모든 서비스 완성 전에도 사용자 피드백을 받습니다. 일정은 API 접근성과 공급사 답변에 따라 결정합니다.

## 다음 작업: P1

- 지원 계정/존을 확인하고 페이지를 포함한 읽기 전용 목록을 대조합니다.
- 서명·전송 인코딩·성공/오류·응답 ID·nullability·안전한 필드 마스크를 확정합니다.
- 서버 목록/상세와 생성 스키마를 비교하고 생성 이력이 없는 import를 설계합니다.
- 오브젝트/NAS/캐시 서비스 작업과 CLI 옵션을 열거하고 인증 후 MCP 도구를 대조합니다.
- 이름·버전·변경/교체·스토리지 소유권·비밀 처리 ADR을 작성합니다.
- P2 전 폐기 가능한 수명주기 테스트 환경, 복구 규칙, 비용 한도를 확정합니다.

## 이슈 준비

각 작업은 계약 Cxx와 검증 Txxx를 연결하는 추적 이슈로 만듭니다.
구현 이슈에는 범위·의존성·API 근거·예상 plan·import ID·양언어 문서·복구·테스트 정리를 포함합니다.
[기능 이슈 템플릿](../../.github/ISSUE_TEMPLATE/capability.md)을 사용합니다.
초기 GitHub 이슈는 이 작업 문서를 연결하며 설계의 기준은 저장소 문서입니다.

## 결정 기록

ADR에는 배경·대안·선택·영향·근거·테스트를 기록합니다.
미확정 사항은 최소 도구 버전, 수정/교체 정책, 연결된 볼륨 소유권, 읽을 수 없는 생성 입력,
집합 허용 목록, Action 이후 state 정합성, 서비스별 인증 스키마입니다.
scaffolding을 진행하려고 알 수 없는 API 동작을 임의 확정하지 않습니다.

## GitHub 작업 이슈

- [P1: API 계약 실측·전체 기능 조사](https://github.com/dokdo2013/terraform-provider-iwinv/issues/1)
- [P2: Compute 첫 구현](https://github.com/dokdo2013/terraform-provider-iwinv/issues/2)
- [P3: 보안 그룹·블록 스토리지](https://github.com/dokdo2013/terraform-provider-iwinv/issues/3)
- [P4: 기타 control-plane 서비스](https://github.com/dokdo2013/terraform-provider-iwinv/issues/4)
- [P5: 서비스 API·Action·Ephemeral](https://github.com/dokdo2013/terraform-provider-iwinv/issues/5)
- [P6: Registry·한영 사용자 가이드](https://github.com/dokdo2013/terraform-provider-iwinv/issues/6)
