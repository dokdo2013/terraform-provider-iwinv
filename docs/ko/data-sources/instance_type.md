---
page_title: "iwinv_instance_type Data Source - iwinv"
subcategory: "Compute"
description: |-
  정확한 서버 상품 ID(API flavor_id) 하나의 표시 이름을 조회합니다.
---

# iwinv_instance_type

[English](../../data-sources/instance_type.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_instance_type-data-source)

정확한 서버 상품 ID(API flavor_id) 하나의 표시 이름을 조회합니다.

개발 버전이며 Registry 릴리스는 아직 없습니다. [Provider 설정](../index.md)을 먼저 확인하세요.

## 사용 예제

```hcl
variable "instance_type_id" {
  type = string
}

data "iwinv_instance_type" "selected" {
  id = var.instance_type_id
}

output "instance_type_name" {
  value = data.iwinv_instance_type.selected.name
}
```

## 입력 인자

`id` (필수 string): [instance_types](instance_types.md)에서 선택한 정확한 flavor_id입니다. Read 시점에 알려진 비어 있지 않은 값이며 `[A-Za-z0-9_-][A-Za-z0-9_.-]*` 형식이어야 합니다. 점을 보존하고 지원하지 않는 경로 문자는 요청 전에 거절합니다.

## 조회 속성

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `id` | `string` | 설정한 정확한 ID이며 응답과 일치하는지 검증합니다. |
| `name` | `string` | API 상품 표시 이름입니다. |

`id`는 필수 입력이며 나머지는 계산 속성입니다.

## 동작과 제약

이름이 비어 있지 않고 ID가 일치하는 행이 정확히 하나여야 합니다. API 성공 응답이 빈 배열이면 상세 조회 실패이며 null 상품이나 생성 허가로 해석하지 않습니다. 첫 행을 선택하거나 미검증 CPU·메모리·가격·용량 필드를 제공하지 않습니다. 실환경 조회·무변경 plan 근거는 이 한정된 계약에 대한 것이며 인스턴스 생성·resize·존별 가용성을 검증하지 않았습니다. 조회 전용 참조로 원격 소유권·import가 없습니다.

조회 전용이며 import는 해당하지 않습니다. API 오류를 빈 결과로 바꾸지 않습니다.

[Example](../../../examples/data-sources/iwinv_catalogs/main.tf) · [Evidence](../../../design/ko/contract-progress.md)

[공식 API](https://iwinv.readme.io/reference/getv1flavorsflavorid)
