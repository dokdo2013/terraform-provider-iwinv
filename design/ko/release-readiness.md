# 릴리스 준비

[English](../en/release-readiness.md) · [목차](../../README.md)

Registry 릴리스는 아직 없습니다. T077은 서명 없는 패키지 준비만 검증합니다. 서명·게시·새 환경의 Registry 설치가 검증되기 전까지 T053은 미완료입니다. 개발 Provider는 Data Source 18개·리소스 7개를 제공하며 실환경 검증의 한정된 범위는 기능 대장을 따릅니다.

## 서명 없는 반복 검증

Go 1.25.8 또는 1.26.1, GoReleaser **2.18.2**, Python 3.10 이상과 실제 Terraform **1.14.2 실행 파일**을 사용합니다. 사용자 홈 디렉터리 설정에 의존하는 버전 관리 도구의 wrapper 경로는 사용하지 않습니다.

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=sign,publish --parallelism=2
cp terraform-registry-manifest.json dist/terraform-provider-iwinv_0.0.0-dev_manifest.json
python3 scripts/check_snapshot.py dist --terraform /absolute/path/to/terraform
python3 -m unittest discover -s scripts -p 'test_check_snapshot.py' -v
```

Snapshot 버전은 태그와 관계없이 `0.0.0-dev`입니다. GoReleaser는 manifest의 출시 파일명으로 체크섬을 계산합니다. 게시를 생략하면 위의 복사 명령으로 동일 파일을 배치해 오프라인 검사합니다. `dist/`는 Git에서 제외합니다. `--clean`은 이전 빌드 결과를 지우므로 보관할 로컬 근거는 먼저 다른 경로에 옮깁니다.

빌드 대상은 Darwin amd64/arm64, Linux amd64/arm64/arm(ARMv6)/s390x, Windows amd64입니다. CGO를 끄고 로컬 경로를 제거하며 Provider 버전을 주입합니다. 이는 교차 컴파일이며 모든 대상의 네이티브 실행을 검증한 것은 아닙니다. 검증기는 정확한 ZIP·내부 파일 집합, 실행 권한, CRC, 체크섬, 프로토콜 6 manifest와 Go 대상 메타데이터를 확인합니다. 합성 테스트는 손상, 체크섬 누락·중복, 잘못된 프로토콜, 추가·경로 이탈 파일, 실행 권한 누락을 거절합니다.

현재 호스트에서는 `-version`을 확인하고 격리된 filesystem mirror의 ZIP으로 실제 `terraform init`을 수행한 뒤 Provider 스키마의 등록 이름을 구현 대장과 비교합니다. dev override, 직접 다운로드 경로, 사용자 캐시, 클라우드 키를 사용하지 않습니다. 서명 없는 mirror 설치는 **서명이나 Registry 검증이 아닙니다**. 임시 설정과 lock 파일은 자동으로 지웁니다. plan/apply나 클라우드 요청은 하지 않습니다.

패키지 workflow는 저장소 읽기 권한만 사용하고 action·도구 버전을 고정합니다. 서명 비밀키나 릴리스 업로드가 없으며 snapshot 명령에서 서명·게시를 명시적으로 생략합니다. 기존 프로토콜 CI는 Terraform 1.14.0/1.14.2를 독립적으로 검증합니다.

## 남은 출시 조건

- 미해결 기능 계약과 정리 실패 T056을 해결하거나 정확한 출시 범위에 반영합니다. 작업에서 만든 웹메일 서비스와 동의하지 않은 MCP 클라이언트 등록도 포함하며, 근거 없이 삭제됐다고 표시하지 않습니다.
- Provider 소개와 등록된 기능별 문서는 한·영으로 마련했습니다. 전체 동작·스키마 검토(T052)와 Registry 렌더링을 마쳐야 하며 파일 존재만으로 합격 처리하지 않습니다.
- 프로젝트 전용 서명키의 보관·복구 방식을 마련하고 `dokdo2013` Registry namespace에 공개키를 등록합니다. 생성 전 현재 허용 알고리즘을 확인합니다. 이번 검증은 키를 생성하거나 업로드하지 않았습니다.
- 출시 조건과 자격증명 보관이 준비되면 권한을 제한한 서명·게시 workflow를 추가합니다. GoReleaser에는 `GPG_FINGERPRINT`를 이용한 체크섬 서명과 draft release를 구성했지만 이 경로는 **실행·검증하지 않았습니다**.
- 변경할 수 없는 semantic version을 정하고 manifest·ZIP·체크섬·바이너리 형식의 분리 GPG 서명을 검증한 뒤 게시 및 새 환경의 Registry 설치를 확인합니다. 이미 게시한 버전을 덮어쓰지 않습니다. 프로토콜 6만으로 CLI 최소 버전이 표현되지 않으므로 최소 Terraform 버전도 별도로 확인합니다.
- 이전 릴리스가 생기면 지원 업그레이드 경로를 검증합니다(T054). 현재 출시된 이전 state 버전이 없으므로 migration 합격을 주장하지 않습니다. workflow 권한·외부 PR 검토(T055)도 완료해야 합니다.

T077 근거: `.goreleaser.yml`, `terraform-registry-manifest.json`, `scripts/check_snapshot.py`, `scripts/test_check_snapshot.py`, `.github/workflows/package.yml` 및 [실행 기록](contract-progress.md).

출처: [HashiCorp 게시 요건](https://developer.hashicorp.com/terraform/registry/providers/publishing), [권장 대상](https://developer.hashicorp.com/terraform/registry/providers/os-arch), [공식 scaffold 설정](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/.goreleaser.yml), [GoReleaser snapshot](https://goreleaser.com/customization/publish/snapshots/).
