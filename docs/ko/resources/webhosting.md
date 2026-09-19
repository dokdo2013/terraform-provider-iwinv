---
page_title: "iwinv_webhosting Resource - iwinv"
subcategory: "호스팅"
description: |-
  초기 쓰기 전용 비밀번호와 명시적인 교체 조건으로 웹호스팅 계정을 관리합니다.
---

# iwinv_webhosting 리소스

[English](../../resources/webhosting.md) · [개발용 설치](../../../design/ko/development.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_webhosting-resource)

공개 호스팅 제어 API를 사용하는 개발용 리소스입니다. Registry 릴리스는 아직 없습니다.
실환경 검증 범위는 공유형 호스팅의 PHP 8.4, 기본/사용자 도메인과 웹방화벽 Y/N입니다.
다른 상품·버전, 실제 서비스 접속과 과금 종료는 추가 검증이 필요합니다.
한 계정의 조회 가능한 생성 속성과 사용자 도메인 전체 map을 소유합니다.
웹 파일·DB 내용·DNS·TLS 인증서·트래픽 초기화·콘솔 전용 변경은 관리하지 않습니다.

## 시작하기

[전체 예제](../../../examples/resources/iwinv_webhosting/main.tf)에 명시적인 상품/서버 카탈로그 ID,
새 영문 6–12자 계정명과 서로 다른 ephemeral 비밀번호 변수 두 개를 입력합니다. [상품](../data-sources/webhosting_products.md)과 [서버](../data-sources/webhosting_servers.md) 목록을 검토해 ID를 선택하세요.
계정명은 정수 서비스 ID와 다릅니다.

```hcl
variable "product_id" { type = string }
variable "server_id" { type = string }
variable "account_name" { type = string }

variable "ftp_password" {
  type      = string
  sensitive = true
  ephemeral = true
}
variable "database_password" {
  type      = string
  sensitive = true
  ephemeral = true
}
resource "iwinv_webhosting" "example" {
  product_id           = var.product_id
  server_id            = var.server_id
  account_name         = var.account_name
  name                 = "tf-example-hosting"
  ftp_password_wo      = var.ftp_password
  database_password_wo = var.database_password
  password_wo_version  = 1

  lifecycle { prevent_destroy = true }
}
```

비밀정보 관리자나 프로세스 환경변수로 비밀번호를 전달하고 Git·일반 output·셸 기록에 실제 값을 넣지 마세요.
두 write-only 입력은 생성·교체에 필요하지만 import와 기존 서비스 조회에는 필요하지 않습니다.
비밀번호는 서로 달라야 하며 공백 없는 출력 가능 ASCII 7–20자, 영문/숫자/특수문자 중 최소 2종류를 사용합니다.
이는 Provider의 지원 입력 범위이며 공급사 비밀번호 경계를 모두 실측했다는 뜻은 아닙니다.

## 설정 변경과 데이터 삭제

공개 API에는 호스팅 수정 경로가 없습니다. 원격 설정 변경은 계정 전체 교체이며 기존 호스팅 데이터를 삭제합니다.
공급사는 삭제 후 같은 계정명을 24시간 동안 재사용할 수 없다고 명시합니다(C29).
따라서 일반 설정 변경의 교체 계획에는 **알려진 새로운 `account_name`**, 서버 선택과 두 초기 비밀번호가 필요합니다.
별칭 변경만으로도 교체가 필요할 수 있습니다. 로컬 timeout만 바꾸면 원격 쓰기를 하지 않습니다.

새 계정을 사용하고 콘텐츠 백업·이전, DNS 전환을 별도로 준비하세요. `create_before_destroy = true`는 새 계정을
먼저 만들 수 있지만 사용자 도메인 재사용을 보장하거나 데이터·DNS를 옮겨주지 않습니다.
예제의 `prevent_destroy`는 실수로 삭제하는 것을 막습니다. 삭제하려면 의도적으로 해당 보호 설정을 수정해야 합니다.
리소스 블록을 설정에서 삭제하면 이 보호도 사라지며, 콘솔·API 삭제를 막지는 않습니다. [동작 범위](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle#prevent_destroy).

**`-replace`, taint, 외부 삭제는 별도의 Terraform Core 동작입니다.** 속성 비교만으로 모든 교체를 감지·차단할 수 없습니다.
삭제 후 같은 계정명 재생성이 실패할 수 있으므로 계획을 확인하고 새 계정명을 사용하세요.
24시간 재시도 루프나 실제로 존재하지 않는 제자리 수정은 제공하지 않습니다.

Write-only 비밀번호 값만 바꿔서는 plan 차이가 생기지 않습니다. `password_wo_version`은 선택적인 양의 정수 로컬 트리거입니다.
이를 바꾸면 새 계정명으로 전체 교체하며 비밀번호만 제자리에서 바꾸는 기능이 아닙니다.
Provider는 비밀번호나 그 해시를 저장하지 않습니다. 설정 파일 자체에 비밀 리터럴이 남지 않도록 ephemeral 변수를 사용하세요.

## 스키마

| 속성 | 타입 | 동작 |
| --- | --- | --- |
| `product_id` | 필수 string | 실제 상품 ID. 변경 시 교체. |
| `account_name` | 필수 string | ASCII 영문 6–12자. 변경 시 교체. |
| `name` | 필수 string | Unicode 코드 포인트 4–32자. 변경 시 교체. |
| `server_id` | 선택 string | 양의 정수 카탈로그 idx의 정확한 십진수 문자열. 생성 필수, Read에 없는 과거 입력. 변경 시 교체. |
| `description` | 선택 + 계산 string | 기본 빈 값, 최대 50자. 명시적 API 빈 문자열은 422이므로 생성 시 생략. |
| `web_firewall_enabled` | 선택 + 계산 bool | 기본 true. API Y/N 명시. 실제 차단 효과의 검증은 아님. |
| `custom_domains` | 선택 + 계산 map(string) | 기본 빈 map. `account_name.iwinv.net`을 제외한 사용자 도메인→폴더 전체 집합. 변경 시 교체. |
| `ftp_password_wo`, `database_password_wo` | 선택 write-only sensitive string | 초기 비밀번호. 생성 필수, import로 복원하지 않음. |
| `password_wo_version` | 선택 int64 | 양의 정수 로컬 교체 트리거. 원격 대응 값 없음. |
| `id` | 계산 string | float64 변환 없이 보존한 정수 service_idx. |
| `status`, `ip_address` | 계산 string | 관찰한 제어면 상태와 IP. |
| `default_domain` | 계산 string | 응답에 실제 존재하는지 확인한 계정명 기반 기본 도메인. |
| `domains` | 계산 map(string) | 서비스가 관리하는 기본 도메인을 포함한 전체 관찰 map. |

이름·설명과 리터럴 HTML entity를 그대로 유지하며 공백 제거나 Unicode 정규화를 하지 않습니다.
Read는 문서화된 기본 서브도메인과 사용자 map을 분리합니다. 기본 도메인이 없으면 소유권을 추측하지 않고 계약 오류로 처리합니다.
외부에서 사용자 도메인을 추가·제거·변경하면 drift이며 교체가 필요할 수 있습니다. 의도적인 외부 변경이면 설정을 맞춰 수용할 수도 있습니다.
폴더는 입력 그대로 전송합니다. 폴더 생성, 검증한 `/` 외의 경로 정규화, 임의 도메인 조합의 가용성은 보장하지 않습니다.

`timeouts`는 create/update/delete 기본 `5m`, read 기본 `1m`이며 양의 duration 문자열만 허용합니다.
`active`는 제어면 상태일 뿐 HTTP/FTP/DB 접속 성공을 뜻하지 않습니다.

## 기존 서비스 가져오기

읽을 수 있는 원격 속성과 일치하도록 설정한 뒤 정확한 정수 서비스 ID로 import합니다.

```sh
terraform import iwinv_webhosting.example 123456789
terraform plan
```

위 숫자는 합성 placeholder입니다. 과거 생성 정보를 모르면 `server_id`, 두 비밀번호와 `password_wo_version`을 생략하세요.
Read로 복원할 수 없습니다. 설명·웹방화벽·사용자 도메인 map은 원격 값과 맞추세요. 기본값은 원하는 설정이며 import한 사실이 아닙니다.
과거/로컬 입력을 나중에 추가하면 교체가 필요한 설정 변경이 될 수 있습니다. 로컬 timeout은 import하지 않습니다.
같은 계정을 두 state에서 동시에 관리하지 마세요.

## 실패 복구와 검증 범위

생성은 한 번 요청하고 명확한 ID를 확보하면 나머지 응답 검증과 active/속성 일치 대기 전에 state에 기록합니다.
실패해도 ID를 남기며 Terraform이 taint할 수 있습니다. 아직 확인되지 않은 생성 ID는 후속 빈 목록만으로 지우지 않습니다.
재apply 전에 실제 상태를 대조하세요. destroy는 생성 여부 미확인 ID에 대해 사전 목록이 비어 있어도 정확한 ID로 삭제를 한 번 요청합니다.
이전에 active였던 서비스는 검증된 전체 목록 부재로 state를 제거할 수 있습니다.
HTTP/API 오류·잘못된 목록·pagination 변경은 부재가 아니며 쓰기를 자동 재전송하지 않습니다.
삭제 접수 뒤 정확한 ID의 부재를 확인하며, 별도의 과금 종료 확인으로 설명하지 않습니다.
존재하는 서비스가 API 목록에서 누락되는 웹메일에는 이 호스팅 계약을 일반화하지 않습니다.

T060은 합성 Core 실패·drift·교체와 실환경 두 서비스 생성/import/무변경 plan, 새 계정 create-before-destroy,
과거 입력 없는 재import, 외부 삭제와 정리를 다룹니다. 저장 plan 압축 내부와 state에서 ephemeral 비밀번호 부재를 검사했습니다.
명시적 taint/`-replace`는 계획만 검사했고 위험한 동일 계정명 재생성을 실행하지 않았습니다.
[근거](../../../design/ko/contract-progress.md)와 [설계 결정](../../../design/ko/webhosting-lifecycle.md)을 참고하세요.

출처: [생성](https://iwinv-hosting.readme.io/reference/웹-호스팅-생성),
[삭제](https://iwinv-hosting.readme.io/reference/웹-호스팅-삭제),
[기본 서브도메인](https://docs.iwinv.kr/service/web-hosting/web-hosting-guide/webhosting_domain/),
[write-only 입력](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments).
