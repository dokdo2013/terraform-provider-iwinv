---
page_title: "iwinv_webhosting_products Data Source - iwinv"
subcategory: "Hosting"
description: |-
  SHARE/SINGLE 필터로 API에 노출된 웹호스팅 상품을 조회합니다.
---

# iwinv_webhosting_products (Data Source)

[English](../../data-sources/webhosting_products.md) · [개발용 설치](../../../design/ko/development.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_webhosting_products-data-source)

서비스를 만들거나 인수하지 않고 상품을 조회합니다. 현재 개발 Provider 전용이며 Registry 릴리스는 없습니다.
목록 포함이나 상품 상태가 생성 성공을 보장하지 않습니다. 상품과 서버 선택지를 직접 검토하세요.

```hcl
data "iwinv_webhosting_products" "shared" {
  type = "SHARE"
}
output "hosting_products" {
  value = data.iwinv_webhosting_products.shared.products
}
```

전체 목록은 `type`을 생략합니다. 정확한 대문자 `SHARE` 또는 `SINGLE`만 지원하고 빈 문자열은 생략과 다릅니다.
상품 이름으로 종류를 추측하지 않고 API 쿼리에 필터를 전달합니다.

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `type` | 선택 string | 정확한 API 필터. 생략/null은 노출된 전체 상품. |
| `ids` | 계산 list(string) | 문자열 사전순으로 정렬한 상품 ID. |
| `products` | 계산 list(object) | `ids`와 같은 순서의 상품 메타데이터. |

각 상품은 `id`, `name`, `status`, `type`, `php_versions`(정렬한 원문 라벨), `allow_custom_domain`,
`enable_domain_folder`, `max_domain_count`, `domain_edit_interval_days`를 포함합니다. 건수와 일 단위 간격은 정수입니다.
표시 문자열은 HTML 엔티티까지 원문을 보존합니다. 가격·VAT·디스크·트래픽은 단위 미검증으로 제외했습니다.
목록 순서는 추천 순서가 아니며 운영 생성에서 무조건 0번을 선택하지 마세요.

빈 목록은 정상입니다. ID 중복, 잘못된 행/필드, 변경된 페이지·건수 메타데이터는 조회 전체를 실패시킵니다.
부분 결과를 완전한 목록으로 반환하지 않고 오류나 HTTP 404를 빈 목록으로 처리하지 않습니다.
읽기 전용이므로 import나 원격 소유권이 없습니다.

선택한 상품 ID로 [서버 선택지](webhosting_servers.md)를 확인하고 [호스팅 리소스](../resources/webhosting.md)에
선택한 서버 ID를 설정하세요. [통합 예제](../../../examples/data-sources/iwinv_webhosting_catalogs/main.tf)는 조회만 합니다.
T061은 잘못된 입력/빈 목록/오류의 합성 검증과 실환경 읽기 후 무변경 plan을 포함합니다.
실제 생성 수명주기 검증 범위는 리소스 가이드의 상품/버전으로 제한됩니다.

출처: [공식 상품 조회](https://iwinv-hosting.readme.io/reference/공유형단독형).
