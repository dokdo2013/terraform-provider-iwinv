---
page_title: "iwinv_webmail_products Data Source - iwinv"
subcategory: "Webmail"
description: |-
  ID가 빈 준비 중 상품을 포함해 웹메일 상품 전체를 조회합니다.
---

# iwinv_webmail_products

[English](../../data-sources/webmail_products.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_webmail_products-data-source)

카탈로그 메타데이터만 읽습니다. 개발용 Provider이며 Registry 릴리스는 아직 없습니다.

```hcl
data "iwinv_webmail_products" "all" {}

output "webmail_products" {
  value = data.iwinv_webmail_products.all.products
}
```

입력 인자나 문서화된 필터는 없습니다. `products`는 계산 객체 목록입니다.

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `product_id` | string | 정확한 상품 ID. 빈 문자열을 보존하며 생성에 사용할 수 있는 ID로 취급하지 않습니다. |
| `name` | string | 상품 이름 원문. ID가 다른 상품의 표시 이름이 같을 수 있습니다. |
| `status` | string | 상품 카탈로그의 가용 상태이며 서비스 준비 상태가 아닙니다. |
| `product_type` | string | `spec.type` 원문이며 서비스 격리를 보장하지 않습니다. |

ID·상품 유형·이름의 사전 순서로 정렬합니다. 빈 ID가 먼저 오므로 첫 번째 상품을 기본값으로 선택하지 마세요.
실측은 SHARE 상품 12개였고, 서로 다른 ID의 사용 가능 상품 4개와 빈 ID의 준비 중 상품 8개였습니다. 모든 행을 보존하며
임의 ID 생성·빈 ID를 기준으로 한 상품 합치기·상품 가용성을 근거로 한 프로비저닝 성공 판단을 하지 않습니다.

실제 응답은 count·페이지 메타데이터 없는 HTTP 200이었습니다. 정상 빈 배열은 허용합니다. 누락/null/문자열이 아닌 ID,
누락되거나 빈 이름·상태·유형, 중복된 비어 있지 않은 ID, 빈 ID이면서 유형·이름까지 같은 모호한 행, 페이지 계약 변경,
선택적인 count의 불일치 및 API 오류는 전체 조회를 실패시킵니다. 실패 시 부분 결과를 반환하지 않습니다. 가격·VAT·디스크·트래픽은
단위·과금 계약을 별도로 검증하기 전까지 제외합니다. 원격 소유권·import는 없고 자격증명이나 메일 내용도 노출하지 않습니다.

**웹메일 서비스·메일 계정 관리 리소스는 아직 지원하지 않습니다.** 기존 수명주기 검사에서 서비스 조회·삭제의 불일치가 나타났고
메일 계정의 독립 Read도 검증하지 못했습니다. 별도 테스트 서비스의 정리 T056도 미해결입니다. 상품 ID가 유효해도 이 제약은
해결되지 않습니다. 이 Data Source는 GET만 수행하며 서비스/계정 생성·메일 발송·DNS 변경은 하지 않습니다.

T075는 어댑터와 Terraform Core에서 빈 ID·서로 다른 준비 중 행·중복 표시 이름·한글/HTML 모양 이름 원문·응답 순서 변경·잘못된/빈/오류
응답을 검사하고, 실환경 전체 조회 후 무변경 plan을 확인합니다. 상품이 추가되면 출력도 달라질 수 있습니다.

[예제](../../../examples/data-sources/iwinv_webmail_products/main.tf) · [근거](../../../design/ko/contract-progress.md)

출처: [공식 상품 조회](https://iwinv-webmail.readme.io/reference/웹-메일-상품-조회).
