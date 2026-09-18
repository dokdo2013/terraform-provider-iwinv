# Contributing / 기여 안내

## 한국어

현재는 설계와 API 계약 검증 준비 단계입니다. 구현을 추가하기 전에 [로드맵](design/ko/roadmap.md)과
[검증 계획](design/ko/verification.md)을 확인해주세요.

1. 기능 대장과 관련 Cxx/Txxx 항목을 찾아 이슈에 연결합니다.
2. API 근거, 지원할 입력·출력, 변경/교체, import, 오류·복구·정리 방법을 제안합니다.
3. 구현 단계에서는 일반 단위 테스트와 실제 API acceptance를 구분합니다.
4. 한국어·영어 설명을 같은 PR에서 업데이트하고 `python3 scripts/check_docs.py`를 실행합니다.
5. 호환성이 바뀌면 ADR과 migration 안내를 작성합니다.

실제 비밀 값·인프라 ID·상태 파일·개인정보·회사 내부 자료를 이슈/PR에 넣지 마세요.
문서에서 복사한 예제의 TLS 검증 비활성화나 비밀 값 출력을 구현에 옮기지 않습니다.
자동화는 별도 허가된 테스트 환경만 사용하며 fork PR에 인증키를 제공하지 않습니다.
기여 파일은 [MPL-2.0](LICENSE)을 따릅니다. 외부 코드는 원 라이선스·고지 조건을 별도로 확인합니다.

## English

This repository is currently preparing design and API contract verification. Read the
[roadmap](design/en/roadmap.md) and [verification plan](design/en/verification.md) first.

1. Link the capability inventory and relevant Cxx/Txxx entries in an issue.
2. Propose API evidence, inputs/outputs, update/replacement, import, errors, recovery and cleanup.
3. Distinguish ordinary unit tests from real API acceptance tests when implementing.
4. Update Korean and English in the same PR and run `python3 scripts/check_docs.py`.
5. Add an ADR and migration guidance for compatibility changes.

Do not publish credentials, real infrastructure IDs, state, personal information or private organizational material.
Do not copy disabled TLS verification or secret printing from examples into implementation.
Automation uses a separately scoped test environment and never supplies credentials to untrusted fork PRs.
Contributions use [MPL-2.0](LICENSE); retain and review applicable licenses/notices for external code separately.
