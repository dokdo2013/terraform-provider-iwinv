# 개발용 Provider 실행

[English](../en/development.md) · [진행 현황](contract-progress.md)

Registry 릴리스는 아직 없습니다. 로컬 바이너리는 존·이미지·상품·SSH 키 Data Source 7개와
[보안 그룹 속성 리소스](../../docs/ko/resources/security_group.md)를 구현합니다. 설계 문서의 인스턴스 예제는 아직 적용할 수 없습니다.

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
- 서명 릴리스, 게시, state migration, 나머지 관리 리소스 acceptance는 후속 단계입니다.

실제 바이너리 alias 검증을 다시 실행하려면 빌드 후 다음 명령을 사용합니다.

```sh
IWINV_LIVE_READ=1 python3 scripts/test_alias_isolation.py
```

## 이미지와 상품 카탈로그

[카탈로그 예제](../../examples/data-sources/iwinv_catalogs/main.tf)는 같은 개발 override와 환경변수 인증을 사용합니다.
`iwinv_images`, `iwinv_instance_types`는 정확한 API ID를 사전순으로 정렬한 `ids` 목록을 제공합니다.
모든 페이지를 조회하며 목록 순서에서 이름·가격·사용 가능 여부를 추론하지 않습니다.
ID를 직접 선택한 다음 예제의 `image_id` 또는 `instance_type_id`를 지정하면 상세 정보를 조회합니다.

| Data Source | 필수 입력 | 계산 출력 |
| --- | --- | --- |
| `iwinv_image` | `id`: 정확한 이미지 ID | `visibility`, `image_type`: API 원문 문자열 |
| `iwinv_instance_type` | `id`: 정확한 flavor ID | `name`: 상품 표시 이름 |

상품 ID의 점은 그대로 보존합니다. 암묵적 필터·최신 이미지 선택·기본 상품 선택은 제공하지 않습니다.
없는 ID, 여러 결과, 요청과 다른 ID가 반환되면 오류입니다.
조회 전용 카탈로그이므로 원격 객체를 소유하지 않으며 import는 해당하지 않습니다.
목록에 있다는 사실만으로 존별 제공 여부·이미지/상품 호환성·잔여 용량·현재 청구 가격을 보장하지 않습니다.
공개 이미지 상세를 실환경 검증했으며 비공개 이미지 응답 변형과 상세 상품 사양은 미검증입니다.

실측한 페이지 크기 10을 사용하고 page/count를 검사하며 중복 ID와 도중에 바뀐 상품 total을 거부합니다.
이미지는 짧은 페이지, 상품은 정확한 total에 도달하면 종료합니다. 이미지의 마지막 페이지가 꽉 차면 다음 페이지까지 조회합니다.
1,000페이지를 넘으면 명시적으로 실패합니다. 도중에 실패해도 부분 목록을 반환하지 않습니다.
API snapshot 토큰이 없어 조회 중 카탈로그 변경을 완전히 배제할 수는 없습니다. 조회를 자동 재시도하지 않습니다.

```sh
TF_ACC=1 IWINV_LIVE_READ=1 go test ./internal/provider -run '^TestAccCatalogs$' -v
```

이 조회 전용 acceptance는 테스트 입력으로만 첫 정렬 ID를 골라 두 상세 정보를 읽고,
후속 plan이 무변경인지 확인합니다. 실제 사용할 이미지·상품을 추천하는 선택 로직이 아닙니다.

## 공통 쓰기 클라이언트 검증

내부 Go 클라이언트는 JSON POST/PUT, multipart POST/PUT, body 없는 DELETE를 지원합니다.
요청은 1 MiB로 제한하며 잘못된 path·field 이름·payload를 전송 전에 거부합니다.
multipart 값은 그대로 전달하므로 API별 percent encoding은 서비스 계층에서 검증 후 적용합니다.
오류 응답·리다이렉트·연결 중단에 대해 자동 재시도하지 않습니다.
성공 HTTP 202는 보존하며, 비동기 완료나 삭제를 뜻한다고 가정하지 않습니다.

Go 1.26.1의 전송 소스에서 자동 replay 경로를 확인했습니다.
클라이언트는 새 HTTP/1 연결을 사용해 HTTP/2 stream 재전송과 재사용 연결에서의 HTTP/1 재전송을 차단합니다.
TLS 검증은 유지하며 추가 연결 비용이 발생합니다. 전송 최적화는 향후 멱등성·명시적 재시도 계약을 검증한 뒤 진행합니다.
[Go HTTP/1 전송](https://cs.opensource.google/go/go/+/refs/tags/go1.26.1:src/net/http/transport.go),
[Go HTTP/2 전송](https://cs.opensource.google/go/go/+/refs/tags/go1.26.1:src/net/http/h2_bundle.go).

아래 테스트는 **조회 전용이 아닙니다**. 인증된 계정에 연결되지 않은 보안 그룹 하나를 만들고 수정·삭제합니다.
명시적으로 지정한 테스트 계정에서만 실행하고, 저장소 밖의 private journal 디렉터리를 지정합니다.
디렉터리는 0700, 생성 파일은 0600이며 생성 응답·ID와 삭제 확인 결과를 보존합니다.
테스트가 실패해도 이번 create에서 얻은 ID만 정리하며, 프로세스 강제 종료 시에는 journal로 수동 복구해야 합니다.

```sh
IWINV_LIVE_WRITE=1 IWINV_TEST_JOURNAL_DIR=/absolute/private/test-journals \
  go test ./internal/client -run '^TestAccControlPlaneWrites$' -v -count=1
```

키는 기존 `IWINV_ACCESS_KEY`/`IWINV_SECRET_KEY` 환경변수로만 전달합니다. CI에서는 이 조건을 켜지 않습니다.
이 테스트는 타입이 있는 network 어댑터를 통한 Go JSON POST/PUT/DELETE, 페이지 목록/정확한 ID 조회, 설명 1회 디코딩, 설명 생략 수정, 빈 값 clear의 전송 전 거부와 정리를 검증합니다.
앞선 ASCII 빈 값 관찰을 이스케이프된 설명으로 일반화하면 안 됩니다. 최신 [계약 근거](contract-progress.md)를 참고하세요.
multipart 성공 수명주기나 Terraform 관리 리소스를 검증했다는 의미는 아닙니다.

## 기존 SSH 키 참조

[SSH 키 예제](../../examples/data-sources/iwinv_ssh_keys/main.tf)는 같은 개발용 override와 환경변수 인증을 사용합니다.
iwinv에 이미 등록된 키의 참조만 조회하며 키 생성·업로드·교체·삭제는 하지 않습니다.
`ssh_key_id`에 사용할 키의 정확한 ID를 지정하세요. 이름 중복은 허용되며 이름으로 자동 선택하지 않습니다.

| Data Source | 입력 | 출력 |
| --- | --- | --- |
| `iwinv_ssh_keys` | 없음 | `ids`: list(string); `keys`: `id`, `name`을 가진 list(object). 둘 다 ID 사전순으로 일치 |
| `iwinv_ssh_key` | 필수 `id`, 정확한 API `ssh_key_id` | Computed `name` |

문서에 목록 API만 있어 단건 조회도 전체 페이지를 읽고 검증합니다.
없는 ID는 오류입니다. 빈 목록은 빈 목록으로 보존하고 오류를 빈 성공 state로 바꾸지 않습니다.
개인키·공개키 본문과 생성 시각은 출력 속성이 아닙니다. 참조 ID와 이름은 Terraform state에 저장됩니다.
읽기 전용이라 import가 적용되지 않습니다. 서버에 키가 실제 설치되었거나 콘솔 전체 계정이 조회됨을 보장하지 않습니다.
검토한 공개 API에 키 생성·삭제 작업이 없는 제약은 C18에 유지합니다.

페이지당 10개를 요청하고 count/page 일치, 중복 ID, 예상 밖 total/page 필드를 검증합니다.
꽉 찬 페이지 뒤에는 다음 페이지도 조회하며 짧은 페이지에서 끝냅니다. 최대 1,000페이지 이후 명시적으로 실패하고 중간 오류에 부분 결과를 반환하지 않습니다.
API에 snapshot token이 없어 동시에 키가 변경되는 상황의 완전한 일관성은 보장할 수 없습니다. 자동 재시도는 하지 않습니다.

```sh
TF_ACC=1 IWINV_LIVE_READ=1 go test ./internal/provider -run '^TestAccSSHKeys$' -v
```

이 실환경 테스트에는 기존 키가 최소 하나 필요합니다. 두 Data Source를 조회하고 후속 무변경 plan을 확인합니다.
첫 정렬 ID는 테스트 입력일 뿐 추천 선택 정책이 아닙니다. 클라우드 객체를 만들거나 수정하지 않습니다.
출처: [공식 SSH 키 목록](https://iwinv-common.readme.io/reference/get_new-endpoint-1-1).

## 보안 그룹 속성

첫 관리 리소스는 [iwinv_security_group](../../docs/ko/resources/security_group.md)입니다.
위 개발용 override를 설정한 후 [리소스 예제](../../examples/resources/iwinv_security_group/main.tf)를 validate하고 plan을 검토한 뒤 적용하세요.
Data Source 예제와 달리 apply가 클라우드 객체를 생성합니다. 사용 후 destroy와 부재 확인까지 진행하세요.
기존 공유 그룹을 테스트 fixture로 쓰지 마세요. 규칙, 연결, 빈 설명 생성/초기화, 연결된 그룹 삭제는 미지원 또는 미검증입니다.

```sh
TF_ACC=1 IWINV_LIVE_TERRAFORM_WRITE=1 \
  IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  go test ./internal/provider -run '^TestAccSecurityGroup$' -v -timeout 12m
```

별도의 실환경 실행 gate이며 같은 환경변수 인증정보가 필요합니다. 공개 CI에서는 켜지 마세요.
테스트 wrapper는 쓰기 전에 기존 그룹 ID 목록을 조회하고 이번 실행의 생성 응답으로 확보한 새 ID에만 변경을 허용합니다.
비공개 mode-0600 대장에 요청 의도, 생성 응답, 확보한 ID, 삭제 시도와 부재 확인을 원자적으로 기록합니다.
테스트 출력/state와 대장에는 계정 식별자가 포함될 수 있으므로 모두 Git 밖에 보관하세요. 원본 로그를 공개 근거로 공유하지 마세요.
실패 후에도 실행하는 정리 절차는 소유한 각 ID를 다시 읽으며, 이전 DELETE를 무작정 반복하지 않습니다.
생성 ID가 불명확하거나 삭제가 확인되지 않았다면 대장을 바탕으로 확인해야 합니다. 테스트 출력이 없다는 사실은 정리 증거가 아닙니다.

지원 속성, ID 유지 수정, 전체 속성을 비교하는 import, 원격 삭제 없이 Terraform state의 소유권만 제거,
영속 state로 재import한 뒤 무변경 plan, 외부 변경/복원, 외부 삭제/재생성과 최종 destroy를 검증합니다. 이번 실행 소유 그룹만 사용합니다.
합성 테스트는 기본값, timeout만 변경, 잘못되거나 unknown인 입력, 반영 지연,
API 오류, 대기 기한 초과와 생성 실패 후 Terraform Core가 ID를 보존하여 정리하는 동작을 별도로 검증합니다.
import state 검증은 이전 Terraform 소유권을 명시적으로 제거하고 다시 가져옵니다. 기존 관리 주소에 그대로 import하는 것은 유효하지 않습니다.
