---
page_title: "iwinv_content_cache_products Data Source - iwinv"
subcategory: "콘텐츠 전송"
description: |-
  null·빈 ID를 보존하며 전체 캐시 상품 행을 조회합니다.
---

# iwinv_content_cache_products (Data Source)

[English](../../data-sources/content_cache_products.md) · [개발용 설치](../../../design/ko/development.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_content_cache_products-data-source)

서비스를 생성하지 않고 콘텐츠 캐시 상품을 조회합니다. 개발 Provider이며 Registry 릴리스는 아직 없습니다.

```hcl
data "iwinv_content_cache_products" "all" {}
data "iwinv_content_cache_products" "shared" {
  product_type = "SHARE"
}
output "cache_products" {
  value = data.iwinv_content_cache_products.shared.products
}
```

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `product_type` | 선택 string | 정확한 `SHARE` 또는 `SINGLE` 필터. 생략하면 보이는 모든 등급을 조회합니다. |
| `products` | computed list(object) | null ID 우선, 이후 ID·등급·이름의 사전순으로 정렬한 전체 행. |

각 행은 nullable `product_id`와 관측 문자열 `name`, `status`, `product_type`을 포함합니다.
null ID는 실제 coming-soon 상품에 존재하며, 빈 ID도 null과 구분하여 보존합니다. 둘 다 서비스 생성에 사용할 수 없습니다.
[`iwinv_content_cache`](../resources/content_cache.md)에 전달할 비어 있지 않은 ID를 직접 검토하세요.
첫 상품을 자동 선택하거나 ID를 만들거나 단순 `ids` 집합으로 줄이지 않습니다. 정렬 순서는 추천이 아니며,
상품이 보이거나 available이라고 해서 생성 자격이 보장되지는 않습니다.

생략/null 필터는 전체 조회입니다. 빈 문자열이나 대소문자가 다른 값은 오류이며, unknown은 Read 전에 확정되어야 합니다.
unknown을 전체 조회로 바꾸지 않습니다. 응답은 요청한 등급과 일치해야 합니다. C31에 따르면 카탈로그 `SHARE`가
생성 서비스에서는 `SINGLE`로 관측될 수 있습니다. 두 값을 그대로 보존하고 격리 수준을 추정하지 않습니다.
가격·디스크·트래픽 등 단위 의존 필드는 검증 전까지 제외합니다.

빈 배열은 정상입니다. ID 누락, string/null 외 ID 타입, 잘못된 행, 비어 있지 않은 ID의 중복, 선택 불가 행의 이름·등급 조합 중복,
필터 불일치, 페이지·건수 메타데이터 변경이나 API 오류는 전체 Read를 실패시킵니다. import나 원격 소유권은 없습니다.
T067은 이 합성 사례와 null ID를 포함한 전체/SHARE/SINGLE 실환경 조회, 후속 무변경 plan을 검증합니다.
모든 상품의 생성이나 콘텐츠·FTP 연결을 검증한 것은 아닙니다.

[읽기 전용 예제](../../../examples/data-sources/iwinv_content_cache_products/main.tf) ·
[검증 근거](../../../design/ko/contract-progress.md)

출처: [공식 캐시 상품 API](https://iwinv-cache.readme.io/reference/컨텐츠-캐시-상품-조회).
