# 한국어·영어 문서 정책

[English](../en/documentation.md) · [목차](../../README.md) · 리비전: 1

한국어와 영어를 동등한 기본 언어로 제공합니다. 루트 README는 한국어로 쓰고 영어 링크를 바로 표시합니다.
설계는 `design/ko`와 `design/en`의 동일한 경로로 관리하며 같은 PR에서 함께 변경합니다.
스키마·API 식별자는 영어를 유지하고 설명은 한국 사용자가 이해하기 쉬운 말로 작성합니다.

## 장기 사용자 가이드

1. 시작 전 확인: 지원 상태, API 활성화, 계정/존 범위, 비용, 서비스별 인증.
2. macOS/Windows/Linux 설치, 버전 제약, Provider alias, 환경변수 인증.
3. 기존 서버부터 시작: 목록 대조, ID 확인, import, 설정 맞추기, 무변경 plan.
4. 테스트 서버 한 대: 이미지/사양 결정, plan, apply, 완료 대기, 확인, 삭제와 과금 종료 확인.
5. 보안 그룹·스토리지·추가 서비스: 소유권과 데이터 보존 관계를 그림과 예제로 설명.
6. 팀 운영: 보호된 remote state, lock 파일, CI 출발 IP, 비밀 관리, quota 공유, 검토·업그레이드.
7. 진단 코드별 문제 해결: IP 차단, 시계 오차, 429, 보이지 않는 존, 생성 결과 불명확, import 차이.
8. Action/Ephemeral: 명시적 실행, 중단·중복 발송 영향과 지원 버전.

기능별 문서에는 제공 버전·사전 조건·최소 예제·입력/출력 표·수정/교체 동작·import·timeout/기본값·
권한·과금/데이터 영향·제약·문제 해결·공식 API 링크가 필요합니다. 제목만 번역하지 않고 동작을 설명합니다.

## 원본과 생성 문서 구조

현재 기능별 페이지는 `docs/`와 `docs/ko/`에서 관리하고 언어 간 직접 링크를 제공합니다. Registry의 언어별 라우팅을 가정하지 않습니다. [전체 스키마 참조](../../docs/ko/guides/schema_reference.md)는 실제 Provider 바이너리에서 한·영으로 생성하며 Provider 설정, 리소스, Data Source, 중첩 속성, timeout 블록과 입력·민감·쓰기 전용 표시를 다룹니다. 스키마 JSON에 담기지 않는 동작·수명주기는 개별 가이드에서 설명합니다. `terraform-plugin-docs`나 Registry 렌더링을 검증했다는 의미는 아닙니다.

소개·기능 페이지의 HCL 예제 52개는 필요한 공통 Provider 설정을 더한 뒤 format과 validate를 통과합니다. 한·영 실행 코드 블록은 같고 같은 전체 예제 디렉터리를 연결합니다. 다른 설계 문서의 제안은 실행 불가 표시를 유지합니다. 기존 이름인 `scripts/check_intro_docs.py`를 그대로 사용하되 현재는 모든 Provider 페이지, 실제 등록 스키마와 기능 대장의 일치, 양언어 스키마 참조를 검사합니다. 계정 자격증명·init·plan·apply 없이 격리된 CLI 설정으로 실행합니다.

스키마 변경 후에는 Provider를 빌드하고 아래 명령으로 재생성한 뒤 diff를 검토하고 쓰기 옵션 없이 다시 검사합니다.

```sh
go build -o bin/terraform-provider-iwinv .
python3 scripts/check_intro_docs.py --provider-dir bin --terraform /absolute/path/to/terraform --write-schema-reference
python3 scripts/check_intro_docs.py --provider-dir bin --terraform /absolute/path/to/terraform
```

사용자 홈 설정에 의존하는 버전 관리 wrapper가 아니라 실제 Terraform 실행 파일을 지정합니다. CI는 Terraform 1.14.0과 1.14.2에서 재생성 없이 검사합니다. T080은 이 구조·예제 검증 범위입니다. 전체 설명의 의미와 Registry 렌더링은 T052의 남은 작업이며, 생성된 표만으로 기본값·검증기·plan modifier·실제 API 동작을 증명하지 않습니다. 양언어 사이트와 Registry 게시는 출시 작업으로 남습니다.

진단에는 검색 가능한 고정 코드와 영어 기술 정보를 사용하고 한국어/영어 해결 문서를 연결합니다.
언어에 따라 식별자가 바뀌거나 번역된 오류 문자열을 파싱하지 않습니다. CLI 언어 옵션은 추후 ADR 대상입니다.
한글 예제에는 합성 이름과 문서용 IP만 쓰고 실제 계정·고객 식별자는 넣지 않습니다.

## 번역·릴리스 검증

- CI에서 양언어 파일과 Cxx/Txxx 식별자 대응을 검사하고 의미 일치는 사람이 검토합니다.
- 문서 생성 재현성, 빌드된 Provider로 모든 예제 검증, 로컬 링크 정상 여부를 확인합니다.
- 한글 이름 정규화·글자/바이트 제한·오류 표시를 fixture로 검증합니다.
- 호환성 변경·state migration·기본값 변경·새 제약은 한·영 changelog로 함께 제공합니다.
- 실제 비밀 응답을 번역 시스템에 넣거나 계정 스크린샷을 공개하지 않습니다.

용어: resource = 리소스; data source = 조회용 데이터 소스; state = Terraform 상태;
drift = 외부 변경에 따른 차이; import = 기존 리소스 편입; replacement = 삭제 후 재생성;
idempotency = 중복 실행해도 결과가 같음; eventual consistency = 반영 지연; ephemeral = 영구 저장하지 않는 임시 값.
