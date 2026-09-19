# 릴리스 준비

[English](../en/release-readiness.md) · [목차](../../README.md)

Registry 릴리스는 아직 없습니다. T077은 서명 없는 패키지 준비, T078은 일회용 키의 서명을 검증합니다. 운영 키 서명·게시·새 환경의 Registry 설치가 검증되기 전까지 T053은 미완료입니다. 개발 Provider는 Data Source 18개·리소스 7개를 제공하며 실환경 검증의 한정된 범위는 기능 대장을 따릅니다.

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

## 일회용 키 서명 검증

GnuPG 2.x(`gpg`, `gpgconf`)를 설치한 뒤 실행합니다.

```sh
python3 scripts/check_signing.py --goreleaser /absolute/path/to/goreleaser --terraform /absolute/path/to/terraform
```

같은 GoReleaser 설정에 `--snapshot --clean --skip=publish`를 전달하여 `dist/`를 다시 빌드합니다. 0700 임시 홈에 유효기간 하루인 RSA-3072 서명키를 만듭니다. 암호 없는 테스트 전용 키이므로 운영 서명키로 사용하면 안 됩니다. 설정된 서명 명령은 SHA-256 기반 바이너리 분리 서명을 만들고, `--no-options`로 사용자 GPG 기본 설정이 armor 등 형식을 바꾸지 못하게 합니다.

별도 키 저장소에는 공개키만 가져옵니다. GPG 성공 결과와 생성한 키의 정확한 fingerprint를 확인한 뒤 서명된 체크섬으로 ZIP 7개와 manifest를 검증합니다. 체크섬 변경, 서명 바이트 변경, ZIP 변경, 서명 누락, armor 형식, 예상과 다른 fingerprint, 알 수 없는 공개키의 7가지 실패를 거부합니다. 원본 결과를 검증한 뒤 네이티브 filesystem mirror 설치를 수행합니다. Terraform의 mirror 설치 자체가 이 GPG 서명을 인증하는 것은 아니며 Registry의 키 신뢰는 미검증입니다.

실행기는 상속된 클라우드·게시 자격증명과 사용자 Git/GPG 설정을 제외하고 공개 Go 캐시만 재사용합니다. 정상 종료와 예외 처리에서 자기 임시 키 저장소의 agent만 종료하고 임시 키 저장소와 `dist/`의 일회용 서명을 제거합니다. 키나 산출물을 업로드하지 않습니다. 강제 프로세스 종료 시에는 로컬 임시 파일을 별도로 정리해야 할 수 있습니다.

패키지 workflow는 저장소 읽기 권한, 고정된 action·도구 버전으로 이 검증을 실행하며 저장된 서명 비밀키나 릴리스 업로드를 사용하지 않습니다. 기존 프로토콜 CI는 Terraform 1.14.0/1.14.2를 독립적으로 검증합니다. 로컬 서명은 Darwin arm64의 GnuPG 2.5.22로 확인했으며 CI는 runner의 GnuPG 버전을 로그에 기록합니다.

## Workflow 권한과 검토 (T055)

2026-09-19 GitHub API로 저장소 설정을 조회했습니다. 기본 workflow 토큰은 읽기 전용이며 workflow의 PR 승인 권한은 꺼져 있고 첫 fork 기여자는 실행 승인이 필요합니다. 저장소 Actions secret과 environment는 없었습니다. 전체 commit SHA 고정을 저장소 정책으로 활성화한 뒤 다시 조회해 반영을 확인했습니다. 허용 Action 범위는 `all`입니다. SHA 고정은 실행 리비전을 고정할 뿐 작성자를 신뢰할 수 있다는 보장은 아닙니다. 이 결과는 조회 시점의 관측값입니다.

현재 workflow 4개는 `main` push와 `pull_request`, `contents: read`, GitHub 호스팅 Ubuntu runner, 인증정보를 남기지 않는 checkout을 사용합니다. 저장된 secret, OIDC 쓰기 권한, 릴리스 environment, `pull_request_target`, `workflow_run`, 게시 단계는 참조하지 않습니다. PR 코드는 네트워크에 접근할 수 있는 runner에서 임의 코드를 실행하므로 정적 검사가 sandbox는 아닙니다. 일반 fork 이벤트의 secret 차단과 토큰 제한은 GitHub 정책을 따릅니다. 이번 점검에서 실제 외부 fork 실행은 수행하지 않았습니다.

`Workflow audit`는 actionlint **1.7.12**로 문법·표현식·셸을, zizmor **1.30.1** 오프라인 pedantic 모드로 workflow 보안 패턴을 검사하며 low 이상 발견 시 실패합니다. zizmor는 `scripts/requirements-workflow.txt`의 정확한 SHA-256 해시를 확인해 임시 가상환경에 wheel만 설치하며 추가 의존성은 없습니다. 분석기에 API 토큰을 전달하지 않으며 온라인 보안 공지나 Action 소유자 검토를 수행하지 않습니다. actionlint에는 Go 모듈 체크섬 검증을 유지합니다. 분석 도구도 의존성이므로 갱신 시 검토가 필요합니다.

첫 검사에서 패키지 작업의 공유 Go 캐시가 산출물 캐시 오염 가능 경로로 표시됐습니다. 해당 GoReleaser 단계는 설치만 하고 서명 실행기는 게시를 생략하므로 실제 게시 릴리스의 취약점이 확인된 것은 아닙니다. 다만 서명 검증 산출물이 다른 실행의 빌드 캐시에 의존하지 않도록 패키지 작업의 공유 캐시 복원·저장을 제거했습니다. 한 실행 안의 Go 캐시는 유지하며 프로토콜 테스트 캐시는 게시와 분리합니다. 변경 후 actionlint와 zizmor의 오프라인 pedantic 검사를 low 기준으로 통과했습니다. 기준 아래 정보 수준의 작업 표시 이름 누락 5건은 남아 있으며 권한에는 영향을 주지 않습니다. 임시 합성 파일의 제목 템플릿 삽입과 잘못된 표현식 context가 실행 없이 거부됐습니다. write-all 권한은 pedantic 모드에서만 발견되어 CI에서 이 모드를 명시합니다.

도구 설치 후 재현 명령:

```sh
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -color
zizmor --offline --no-progress --persona pedantic --min-severity low .github/workflows
```

zizmor는 해시를 고정한 요구사항 파일로 설치한 1.30.1을 사용합니다. 정확한 격리 설치 명령은 workflow에 있습니다. 정적 검사 성공이 런타임 secret 격리, 의존성 안전성, 릴리스 권한을 증명하지는 않습니다. 실제 외부 fork 실행과 향후 운영 서명·게시 workflow 검토가 남아 있어 T055는 진행 중입니다. 운영 키를 추가하기 전에 보호 environment, 변경 불가능한 버전·태그 선택, 게시 job의 최소 권한, PR 산출물·캐시와의 분리, 키 정리, 릴리스 실패 복구를 검토해야 합니다. 현재 PR job에 해당 권한을 부여하지 않습니다.

출처: [GitHub 저장소 Actions 설정](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository), [GitHub 권한 API](https://docs.github.com/en/rest/actions/permissions), [zizmor 실행 모드와 한계](https://docs.zizmor.sh/usage/), [actionlint](https://github.com/rhysd/actionlint).

## 남은 출시 조건

- 미해결 기능 계약과 정리 실패 T056을 해결하거나 정확한 출시 범위에 반영합니다. 작업에서 만든 웹메일 서비스와 동의하지 않은 MCP 클라이언트 등록도 포함하며, 근거 없이 삭제됐다고 표시하지 않습니다.
- Provider 소개와 등록된 기능별 문서는 한·영으로 마련했습니다. 공식 형식 검사와 본문 56개 미리보기를 통과했으며 전체 설명 일치·게시된 Registry 메뉴와 링크 검증(T052)이 남았습니다. 정확한 미리보기 시점·범위는 문서 정책을 따릅니다.
- 프로젝트 전용 서명키의 보관·복구 방식을 마련하고 `dokdo2013` Registry namespace에 공개키를 등록합니다. 생성 전 현재 허용 알고리즘을 확인합니다. 운영 키는 생성·업로드하지 않았고 테스트 키는 삭제합니다.
- 출시 조건과 자격증명 보관이 준비되면 권한을 제한한 서명·게시 workflow를 추가합니다. `GPG_FINGERPRINT`를 이용한 체크섬 서명은 일회용 키로 실행했습니다. 운영 키 로딩과 draft release 업로드는 **실행·검증하지 않았습니다**.
- 변경할 수 없는 semantic version을 정하고 manifest·ZIP·체크섬·바이너리 형식의 분리 GPG 서명을 검증한 뒤 게시 및 새 환경의 Registry 설치를 확인합니다. 이미 게시한 버전을 덮어쓰지 않습니다. 프로토콜 6만으로 CLI 최소 버전이 표현되지 않으므로 최소 Terraform 버전도 별도로 확인합니다.
- 이전 릴리스가 생기면 지원 업그레이드 경로를 검증합니다(T054). 현재 출시된 이전 state 버전이 없으므로 migration 합격을 주장하지 않습니다. workflow 권한·외부 PR 검토(T055)도 완료해야 합니다.

T077 근거: `.goreleaser.yml`, `terraform-registry-manifest.json`, `scripts/check_snapshot.py`, `scripts/test_check_snapshot.py`, `.github/workflows/package.yml` 및 [실행 기록](contract-progress.md).

T078 근거: `scripts/check_signing.py`, `.goreleaser.yml`, `.github/workflows/package.yml` 및 같은 실행 기록입니다. [GoReleaser 서명](https://goreleaser.com/customization/sign/sign/)과 [GnuPG 테스트 키 생성](https://www.gnupg.org/documentation/manuals/gnupg/Unattended-GPG-key-generation.html)을 참고하세요.

출처: [HashiCorp 게시 요건](https://developer.hashicorp.com/terraform/registry/providers/publishing), [권장 대상](https://developer.hashicorp.com/terraform/registry/providers/os-arch), [공식 scaffold 설정](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/.goreleaser.yml), [GoReleaser snapshot](https://goreleaser.com/customization/publish/snapshots/).
