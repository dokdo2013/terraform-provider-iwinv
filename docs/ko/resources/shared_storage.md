---
page_title: "iwinv_shared_storage 리소스 - iwinv"
subcategory: "Storage"
description: |-
  API NAS 서비스와 전체 IPv4 권한 맵을 관리합니다.
---

# iwinv_shared_storage (Resource)

[English](../../resources/shared_storage.md) · [개발 버전 설치](../../../design/ko/development.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_shared_storage-resource)

API NAS의 control-plane을 관리하는 개발 리소스이며 Registry 릴리스는 아직 없습니다.
스토리지 마운트·파일 접근·마운트 안내 실행·tenant 인증정보·데이터 이전은 수행하지 않습니다.

## 예제

사용 가능한 상품을 검토하고 새 공유 이름을 지정하세요. [전체 예제](../../../examples/resources/iwinv_shared_storage/main.tf)의
IP는 문서용 주소이므로 실제 허용할 클라이언트 IPv4로 바꿔야 합니다.

```hcl
variable "product_id" { type = string }
variable "share_name" { type = string }

resource "iwinv_shared_storage" "example" {
  product_id  = var.product_id
  share_name  = var.share_name
  name        = "tf-example-storage"
  description = "Shared application files"
  size_gb     = 100
  allowed_ips = {
    "192.0.2.10" = "RW"
    "192.0.2.11" = "RO"
  }

  lifecycle { prevent_destroy = true }
}
```

## 스키마와 소유 범위

| 속성 | 타입 | 동작 |
| --- | --- | --- |
| `product_id` | 필수 string | 정확한 비어 있지 않은 생성 ID. 변경하면 교체하며 카탈로그 노출만으로 생성 자격을 보장하지 않습니다. |
| `share_name` | 선택 string | 생성 시 필수인 6–20자 ASCII 영숫자 공유 이름. 조회할 수 없는 생성 이력이므로 import에서는 생략합니다. 추가·변경·제거하면 교체합니다. |
| `name` | 필수 string | 원문 별칭, 유니코드 4–32자. 변경하면 교체합니다. |
| `description` | 선택 + 계산 string | 기본 빈 문자열, 최대 50자. 빈 값은 생성 요청에서 생략하며 변경하면 교체합니다. |
| `size_gb` | 필수 int64 | 문서상 100–2000 GB. 변경은 전체 서비스 교체이며 제자리 증설이 아닙니다. |
| `allowed_ips` | 필수 map(string) | 정규 IPv4 호스트를 정확한 `RO`/`RW`에 연결하는 비어 있지 않은 전체 맵. 제자리 수정합니다. |
| `id` | 계산 string | 정확한 양의 십진수 `service_idx`, import ID. |
| `status` | 계산 string | 관측한 control-plane 상태. active만으로 파일 접근·권한 강제를 보장하지 않습니다. |
| `address` | 계산 string | 관측한 도메인. 프로토콜·포트·DNS 소유권을 추정하지 않습니다. |
| `mount_info` | 계산 string | 관측 문자열. 실행하거나 공유 이름 이력을 역산하지 않습니다. |

로컬 `timeouts` 기본값은 create/update/delete `5m`, read `1m`이며 양수여야 합니다.
시간 제한만 바꾸면 원격 쓰기는 없습니다. 이름·설명은 한글과 HTML처럼 보이는 문자열을 그대로 보존합니다.

한 리소스가 **전체 권한 맵**을 소유합니다. PUT은 생략된 호스트를 제거하고 유지된 호스트의 권한도 바꿉니다.
개별 항목을 여러 리소스·state로 나누지 마세요. 빈 맵·CIDR·IPv6·null 권한·다른 권한 표기는 거부합니다.
외부 변경은 drift로 처리해 설정한 권한을 복구합니다. API 재조회는 NFS 권한 강제 검증을 대신하지 않습니다.

[읽기 전용 상품 카탈로그](../data-sources/shared_storage_products.md)로 정확한 ID와 문서상 용량 범위를 검토할 수 있습니다.

## 교체와 import

권한만 제자리 수정합니다. 상품·공유 이름 이력·이름·설명·용량 변경은 스토리지를 교체하고 삭제합니다.
교체에는 알려진 새 공유 이름이 필요합니다. NAS 이름 재사용 규칙은 미검증이며 호스팅·캐시의 재사용 시간을 적용하지 않습니다.
`create_before_destroy`는 새 공유를 먼저 만들 수 있지만 **데이터 복사나 클라이언트 재설정을 하지 않습니다**.
예제의 `prevent_destroy`는 명시적으로 제거하기 전까지 삭제를 막습니다. 교체 전 백업과 이전 계획을 준비하세요.
명시적인 taint/`-replace`는 일반 속성 비교를 우회할 수 있으므로 적용 전 새 이름을 지정하세요.

```sh
terraform import iwinv_shared_storage.example 123456789
terraform plan
```

위 ID는 가상 값입니다. 실제 상품·이름·설명·용량·권한 맵을 맞추고 조회할 수 없는 `share_name`은 생략하세요.
import는 모든 조회 가능 속성을 복원하며 생성 이력 없이 무변경 plan과 권한 수정을 지원합니다.
import 설정에 공유 이름을 넣으면 의도적으로 교체를 요청합니다. 한 서비스는 하나의 Terraform state에서 관리하세요.

## 실패 복구

생성은 POST 한 번을 보내고 응답의 나머지를 검증하기 전에 정확한 ID부터 state에 보존합니다.
이후 active와 조회 가능한 설정의 일치를 기다립니다. 초기화 실패나 가시성 지연에도 ID를 보존합니다.
한 번도 검증하지 못한 서비스가 조회되지 않으면 Read 진단을 반환하며, 기존 active 서비스는 검증된 ID 부재 후 state에서 제거할 수 있습니다.
잘못된 목록·중복 ID·HTTP 오류를 부재로 간주하지 않습니다.

권한 쓰기 전 정확한 부모를 읽고 조회 가능한 설정이 이전 state와 같은지 확인합니다.
동시 변경·부재·조회 실패 시 쓰기를 멈춥니다. 수정 실패는 이전 state를 보존합니다.
삭제는 성공 접수와 검증된 부재를 모두 요구합니다. busy·전송 오류를 포함한 NAS 쓰기를 자동 재시도하지 않습니다.
불확실한 결과는 다음 쓰기 전 조회·대조하고, 실패한 생성의 state는 Core 교체 전에 확인하세요.
API 정리만으로 독립적인 과금 종료까지 입증하지는 않습니다.

[수명주기 설계](../../../design/ko/nas-lifecycle.md)와 [검증 근거](../../../design/ko/contract-progress.md)를 참고하세요.
출처: [생성](https://iwinv-api-nas.readme.io/reference/공유-스토리지-생성),
[권한](https://iwinv-api-nas.readme.io/reference/접근-허용-ip-추가),
[삭제](https://iwinv-api-nas.readme.io/reference/공유-스토리지-삭제).
