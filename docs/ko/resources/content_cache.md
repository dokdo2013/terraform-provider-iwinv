---
page_title: "iwinv_content_cache 리소스 - iwinv"
subcategory: "콘텐츠 전송"
description: |-
  콘텐츠 캐시 서비스와 전체 리퍼러 집합을 관리합니다.
---

# iwinv_content_cache (리소스)

[English](../../resources/content_cache.md) · [개발용 설치](../../../design/ko/development.md)

`cache_lite`로 검증한 개발용 control-plane 리소스이며 Registry 릴리스는 아직 없습니다.
서비스와 전체 리퍼러 목록을 관리합니다. FTP 콘텐츠, 테넌트 API 인증정보, purge, DNS 레코드,
HTTPS 설정과 과금은 이 리소스의 검증 범위에 포함하지 않습니다. 다른 상품은 별도 acceptance가 필요합니다.

## 예제

[전체 예제](../../../examples/resources/iwinv_content_cache/main.tf)에 검토한 상품 ID, 새 계정명, ephemeral 비밀번호를 전달합니다.
비밀번호는 HCL 리터럴이나 Git 파일 대신 비공개 실행 환경으로 전달하세요.

```hcl
variable "ftp_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "iwinv_content_cache" "example" {
  product_id          = var.product_id
  account_name        = var.account_name
  name                = "tf-example-cache"
  allowed_referrers   = ["media.example.com", "www.example.com"]
  ftp_password_wo      = var.ftp_password
  password_wo_version = 1

  lifecycle { prevent_destroy = true }
}
```

프로젝트 기준은 Terraform >=1.14.0입니다. `sensitive`만으로 저장을 막을 수 없으므로 ephemeral 변수와
write-only 인자를 함께 사용하세요. 비밀번호는 설정에서 읽으며 리소스 plan/state에 저장하지 않습니다.
설정에 비밀번호를 직접 쓰면 저장된 산출물에 설정 원문이 포함될 수 있습니다.

## 스키마와 소유권

| 속성 | 타입 | 동작 |
| --- | --- | --- |
| `product_id` | 필수 string | 정확한 비어 있지 않은 생성 ID. 변경 시 교체. 카탈로그 null ID로 생성할 수 없습니다. |
| `account_name` | 필수 string | 조회 가능한 계정명, ASCII 영문·숫자 6–12자. 변경 시 교체. |
| `name` | 필수 string | 별칭, Unicode 코드 포인트 4–32자. 변경 시 교체. |
| `description` | 선택 + computed string | 기본 빈 문자열, 최대 50자. 빈 값은 생성 요청에서 생략. 변경 시 교체. |
| `allowed_referrers` | 선택 + computed set(string) | 기본 빈 집합. 소문자 ASCII DNS 호스트명 전체 목록. 비어 있지 않은 변경은 제자리 수정, 초기화는 교체. |
| `ftp_password_wo` | 선택 sensitive write-only string | 생성·교체에 필수. 공백 없는 출력 가능 ASCII 7–20자, 영문·숫자·기호 중 두 종류 이상. 이 값만 바꾸면 쓰기가 발생하지 않습니다. |
| `password_wo_version` | 선택 int64 | 양수인 로컬 버전. 추가·변경·제거 시 서비스 전체 교체이며 비밀번호 제자리 회전이 아닙니다. import 시 모르는 이력은 생략합니다. |
| `id` | computed string | 정확한 양의 10진수 `service_idx`, import ID. |
| `status` | computed string | 조회된 control-plane 상태. active가 쓰기 잠금 해제나 FTP·콘텐츠 연결 성공을 보장하지 않습니다. |
| `ip_address` | computed string | 조회된 서비스 IP. |
| `domain_name` | computed string | 조회된 도메인 문자열. 포트·프로토콜·DNS 관리 권한을 추정하지 않습니다. |
| `product_type` | computed string | 조회된 `spec.type`. SHARE 카탈로그 상품에서 SINGLE일 수 있으며 격리 보장이 아닙니다(C31). |

로컬 `timeouts` 기본값은 create/update/delete `5m`, read `1m`이며 양수여야 합니다.
timeout만 변경하면 원격 쓰기를 하지 않습니다. 이름·설명은 한글과 HTML처럼 보이는 문자를 포함해 원문을 보존합니다.

API의 “리퍼러 추가”는 **전체 목록 교체**입니다(C22). 하나의 리소스가 집합 전체를 소유해야 합니다.
개별 리퍼러로 분리하거나 다른 state에서 같은 목록을 관리하지 마세요. 외부 변경은 drift이며 설정한 집합으로 복원합니다.
순서는 무관합니다. wildcard·IP·URL·경로·포트·IDN·끝점·null 원소는 쓰기 전에 거절합니다.
미지원 값이 포함된 기존 목록은 원소를 버리지 않고 오류로 알립니다. API 조회만으로 실제 HTTP 필터링을 검증하지는 않습니다.

## 빈 집합, 교체, import

생성 API는 리퍼러 인자를 무시하므로 먼저 부모 ID를 저장하고 서비스 조회를 검증한 뒤 비어 있지 않은 목록을 별도 설정합니다.
처음부터 빈 집합이면 PUT이 필요 없습니다. API는 빈 PUT을 거절하므로 기존 목록을 비우면 **서비스와 콘텐츠 전체를 삭제·교체**합니다.
이때 확정된 다른 계정명과 초기 비밀번호가 필요합니다. 기존 집합이 비어 있지 않고 새 집합 전체가 unknown이면 초기화 가능성을
고려해 보수적으로 교체합니다. 집합이 비어 있지 않음이 확정되고 원소만 unknown이면 값이 확정된 뒤 수정할 수 있습니다.
plan의 교체를 확인하세요. 예제의 `prevent_destroy`는 사용자가 명시적으로 제거하기 전까지 교체를 차단합니다.

이름·상품·계정·설명·버전 변경도 교체입니다. 삭제는 복구할 수 없고 공급사는 삭제한 계정명 재사용을 **24시간** 제한합니다.
매번 새 계정명을 사용하세요. `create_before_destroy`는 새 계정을 먼저 만들지만 콘텐츠나 접속 클라이언트를 옮기지는 않습니다.
명시적인 `-replace`, taint, 외부 삭제는 속성 비교를 우회할 수 있는 Core 경로입니다. 같은 이름의 재생성을 실행하지 않고 plan만 검증했습니다.
실제 적용 전에 새 계정명과 비밀번호를 준비해야 합니다.

조회 가능한 설정을 기존 서비스와 맞춘 뒤 import합니다.

```sh
terraform import iwinv_content_cache.example 123456789
terraform plan
```

위 ID는 가상 예시입니다. import는 실제 계정·상품·이름·설명·전체 리퍼러와 computed 필드를 복원합니다.
`ftp_password_wo`와 `password_wo_version`은 생략하세요. 비밀번호와 로컬 이력은 복원할 수 없습니다.
설정이 일치하면 plan은 비어 있고, 리퍼러 제자리 수정에도 비밀번호가 필요 없습니다.
교체 의도가 없다면 생성 예제의 버전 `1`을 import 설정에 남기지 마세요. 서비스 하나는 state 하나만 소유해야 합니다.

## 실패 복구

생성은 POST 한 번이며 나머지 응답 검증 전에 정확한 ID를 저장합니다. 초기 설정 실패나 조회 지연에도 ID를 유지합니다.
아직 검증된 적 없는 서비스가 목록에 없으면 Read 오류를 내고, 과거 active 서비스의 정확한 ID 부재가 확인된 경우에만 state에서 제거합니다.
부분·잘못된 목록, API 오류, 일반 HTTP 404는 삭제로 간주하지 않습니다. Core는 생성 실패를 taint할 수 있으므로 보존된 ID를
확인하고 실제 상태를 대조한 뒤 교체 여부를 결정하세요.

캐시 PUT/DELETE의 정확한 “다른 작업 진행중” 거절에만 제한 시간 내 재시도를 허용합니다. 매번 정확한 ID를 다시 조회하여
부모 전체가 그대로인지 확인합니다. 변경·부재·조회 실패·기한 만료 시 중단합니다. 접수된 쓰기, 일반 오류, 결과가 불명확한
전송 실패, 생성 요청은 자동 재전송하지 않습니다. 수정·삭제 대기 실패에는 기존 state를 유지하므로 refresh 후 복구를 판단하세요.
삭제 접수와 정확한 ID 부재는 control-plane 정리 증거이며 과금 종료의 독립적인 확인은 아닙니다.

[수명주기 설계](../../../design/ko/cache-lifecycle.md)와 [검증 근거](../../../design/ko/contract-progress.md)를 참고하세요.
별도로 미해결인 웹메일 정리 문제는 캐시 acceptance로 해결되지 않습니다.

출처: [생성](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-생성),
[리퍼러](https://iwinv-cache.readme.io/reference/레퍼러-추가),
[삭제](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-삭제).
