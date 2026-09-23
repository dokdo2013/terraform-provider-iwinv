---
page_title: "iwinv_instance_types Data Source - iwinv"
subcategory: "Compute"
description: |-
  flavors API가 제공하는 정확한 서버 상품 ID를 조회합니다.
---

# iwinv_instance_types

[English](../../data-sources/instance_types.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_instance_types-data-source)

flavors API가 제공하는 정확한 서버 상품 ID를 조회합니다.

개발 버전이며 Registry 릴리스는 아직 없습니다. [Provider 설정](../index.md)을 먼저 확인하세요.

## 사용 예제

```hcl
data "iwinv_instance_types" "available" {}

output "instance_type_ids" {
  value = data.iwinv_instance_types.available.ids
}
```

## 입력 인자

서비스별 입력 인자는 없습니다. 존·가격·용량 필터는 구현하지 않았습니다.

## 조회 속성

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `ids` | `list(string)` | 전체 페이지의 정확한 flavor_id를 사전순으로 정렬합니다. |

모든 속성은 계산 속성이므로 설정에 값을 지정하지 않습니다.

## 동작과 제약

ID의 점 등 허용 문자를 그대로 유지하며 AWS 인스턴스 타입으로 변환하지 않습니다. 페이지당 10개를 읽고 count·페이지 번호·크기·일관된 total을 검증합니다. total에 도달해야 완료하며 중복 ID·조회 중 total 변경·조기 종료 페이지·중간 오류는 전체 결과를 거절합니다. 최대 1,000페이지이고 snapshot 토큰이 없어 동시 변경의 원자적 조회를 보장하지 않습니다. 정상적인 total 0의 빈 응답은 빈 목록입니다. 개별 표시 이름은 [instance_type](instance_type.md)으로 조회하세요. 목록 순번으로 상품을 고르거나 노출 여부를 quota·재고·가격·존 호환성으로 해석하지 마세요. 실측에서 상품 API에 일반 존 필터를 지정해도 그 존에 매핑되지 않은 상품 행이 반환됐으므로, 필터 결과만으로 생성 가능성을 판단하지 마세요.

조회 전용이며 import는 해당하지 않습니다. API 오류를 빈 결과로 바꾸지 않습니다.

[Example](../../../examples/data-sources/iwinv_catalogs/main.tf) · [Evidence](../../../design/ko/contract-progress.md)

[공식 API](https://iwinv.readme.io/reference/getv1flavors)
