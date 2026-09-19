# 개발용 Provider 실행

[English](../en/development.md) · [진행 현황](contract-progress.md)

Registry 릴리스는 아직 없습니다. 로컬 바이너리는 `iwinv_availability_zones`만 구현합니다.
관리 리소스와 수명주기 작업은 등록하지 않았습니다. 설계 문서의 인스턴스 예제는 아직 적용하면 안 됩니다.

## 빌드와 검증

Go >=1.25.8, Terraform >=1.14.0이 필요합니다. 의존성 버전은 [go.mod](../../go.mod)에 고정되어 있습니다.

```sh
go build -o bin/terraform-provider-iwinv .
go test -race ./...
go vet ./...
python3 scripts/check_docs.py
IWINV_PROTOCOL_TEST=1 go test ./internal/provider -run TestProtocol -v
```

프로토콜 테스트는 합성 API fixture를 사용합니다. Terraform 실행 파일을 찾지 못하면
`TF_ACC_TERRAFORM_PATH`에 실행 파일의 절대 경로를 지정합니다.

아래 내용으로 **별도의 로컬** Terraform CLI 설정 파일을 만듭니다. 경로는 빌드한 바이너리가 있는
디렉터리의 절대 경로로 바꿉니다. 기존 개인 CLI 설정은 덮어쓰지 않습니다.

```hcl
provider_installation {
  dev_overrides {
    "dokdo2013/iwinv" = "/absolute/path/to/terraform-provider-iwinv/bin"
  }
  direct {}
}
```

`TF_CLI_CONFIG_FILE`에 이 파일 경로를 지정합니다. 개발 override를 사용하는 이 예제에서는
`terraform init`을 생략합니다. Provider가 게시되지 않아 Registry 설치는 실패합니다.

```sh
terraform -chdir=examples/data-sources/iwinv_availability_zones validate
terraform -chdir=examples/data-sources/iwinv_availability_zones plan
```

validate에는 인증정보가 필요하지 않습니다. plan에는 비밀정보 관리자/환경변수로 주입한
control-plane 키 `IWINV_ACCESS_KEY`, `IWINV_SECRET_KEY`가 필요합니다.
실제 키를 예제·셸 기록·`.tfvars`·GitHub 이슈·커밋 파일에 넣지 마세요.
Provider의 `access_key`, `secret_key` 인자는 각각 환경변수보다 우선합니다.
빈 값 또는 unknown을 명시하면 환경변수의 다른 계정으로 조용히 전환하지 않습니다.

## Availability-zone Data Source

```hcl
data "iwinv_availability_zones" "available" {}

output "zone_ids" {
  value = data.iwinv_availability_zones.available.zone_ids
}
```

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `zone_ids` | list(string), computed | 사전순으로 정렬한 정확한 API ID |
| `names` | list(string), computed | 같은 순서의 표시 이름 |
| `zones` | list(object), computed | 존별 `id`, `name`, `status` |

임의 입력 인자·region 기본값·ID 변환·상태 필터를 만들지 않습니다. 잘못된 행, 일치하지 않는 count,
중복 ID는 오류로 처리합니다. 빈 배열은 빈 카탈로그로 허용하지만 API 오류를 빈 목록으로 바꾸지는 않습니다.
Data Source이므로 import는 해당하지 않습니다. 조회만 수행하며 존을 관리하거나
콘솔의 모든 서버가 API에 노출된다는 것을 보장하지 않습니다.
한·영 문서는 같은 [실행 예제](../../examples/data-sources/iwinv_availability_zones/main.tf)를 사용합니다.

## 명시적인 실환경 읽기 전용 acceptance

```sh
TF_ACC=1 IWINV_LIVE_READ=1 go test ./internal/provider -run '^TestAccAvailabilityZones$' -v
```

Terraform을 통해 존 API를 읽고 결과가 비어 있지 않은지, 후속 plan이 무변경인지 확인합니다.
클라우드 리소스는 생성하지 않습니다. CI에서는 이 실행 조건을 켜거나 클라우드 키를 제공하지 않습니다.
plugin-testing의 재연결 방식 alias만으로 계정 격리를 증명하지 않습니다. 모의 클라이언트 격리 테스트는
해당 도구가 문서화한 별도 factory를 사용합니다. 실제 바이너리에서는 정상 키의 기본 Provider가 조회에 성공하고 합성 오류 키의 alias만 거부되는 것을 확인했습니다. 서로 다른 정상 계정 두 개의 대조는 아닙니다.

## 알려진 제약

- 통제된 multipart 인스턴스 생성 실험이 HTTP 500 / `DEV_CHECK_RETURN`으로 끝났고 ID가 반환되지 않았습니다.
  자동 재시도하지 않았으며 직후와 지연 후의 API 목록은 비어 있었습니다. 수명주기 성공이나 과금 검증 근거는 아닙니다.
- Object Storage/NAS/Cache/메시징의 서비스별 키와 MCP OAuth 인증 후 도구 조사는 남아 있습니다.
- 서명 릴리스, 게시, state migration, 관리 리소스 acceptance는 후속 단계입니다.

실제 바이너리 alias 검증을 다시 실행하려면 빌드 후 다음 명령을 사용합니다.

```sh
IWINV_LIVE_READ=1 python3 scripts/test_alias_isolation.py
```
