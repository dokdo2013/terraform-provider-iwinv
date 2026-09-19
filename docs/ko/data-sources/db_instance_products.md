---
page_title: "iwinv_db_instance_products Data Source - iwinv"
subcategory: "Database"
description: |-
  빈 ID와 반복 상품 ID를 보존하며 DBMS 상품을 조회합니다.
---

# iwinv_db_instance_products (Data Source)

[English](../../data-sources/db_instance_products.md) · [개발용 설치](../../../design/ko/development.md)

DB를 만들지 않고 DBMS 상품 카탈로그를 조회합니다. 개발용 Provider이며 아직 Registry 릴리스는 없습니다.

```hcl
data "iwinv_db_instance_products" "all" {}
data "iwinv_db_instance_products" "redis" {
  product_type = "STD"
  engine       = "redis"
}
output "db_products" {
  value = data.iwinv_db_instance_products.redis.products
}
```

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `product_type` | optional string | 정확한 `STD` 또는 `HM` 조회 필터. 생략하면 다른 등급 표기를 포함한 전체 목록입니다. |
| `engine` | optional string | 정확한 `MySQL`, `MariaDB`, `mongoDB`, `MS-SQL`, `redis`, `PostgreSQL` API 조회 필터. |
| `products` | computed list(object) | 상품 ID·등급·버전 순으로 사전식 정렬한 전체 행. |

생략/null은 필터 없음이며 빈 문자열은 오류입니다. unknown은 Read 전에 확정해야 하며 전체 조회로 대체하지 않습니다.
각 행은 `product_id`, `name`, `status`, `product_type`, `engine_version`의 관측 문자열을 그대로 담습니다.
응답에는 엔진 식별자가 별도로 없으므로 `engine`은 조회 입력이고 `product_type`은 등급입니다.
가격/VAT와 CPU·메모리·디스크 값은 단위와 의미를 검증하기 전까지 제외합니다.

**생성할 상품 ID를 직접 검토하세요.** C30에는 available인데 ID가 빈 행과 버전 간 동일 ID가 반복되는 현상이 기록돼 있습니다.
행을 조용히 누락하거나 ID로 중복 제거하거나 식별자를 만들어내지 않고 모두 보존합니다.
따라서 단순 `ids` 집합이나 첫 항목 자동 선택을 제공하지 않습니다. 정렬 순서는 가용성·버전 추천이 아닙니다.
ID의 모호성을 확인할 때는 전체 카탈로그도 검토하세요. 필터된 행만 보면 같은 ID를 가진 다른 버전이 숨겨질 수 있습니다.
빈 ID로는 리소스를 생성할 수 없습니다.

[DBMS 리소스](../resources/db_instance.md)는 `product_id`를 받지만 공개 POST API에는 버전 선택 인자가 없습니다.
엔진 필터는 생성 시 엔진/버전 제약으로 전달되지 않습니다. 카탈로그의 상태도 생성 자격을 보장하지 않으며,
특정 행이 보인다는 이유로 해당 엔진 버전의 생성을 약속할 수 없습니다.

빈 배열은 정상입니다. 누락/null ID, 잘못된 행, 완전히 같은 ID/등급/버전 조합, 등급 필터 불일치와 바뀐 페이지/건수
메타데이터는 전체 Read를 실패시킵니다. HTTP/API 오류를 빈 목록으로 바꾸지 않습니다. 조회 전용이므로 import나 원격 소유권이 없습니다.

T064는 합성 오류/빈 결과/unknown·잘못된 필터, 빈 ID·중복 ID 보존과 실제 조회 후 무변경 plan을 검증합니다.
실환경 범위는 전체 목록, 엔진 필터 6개, 등급 필터 2개와 STD/redis 조합입니다. 모든 엔진 상품을 생성하거나
엔진 식별·접속을 독립 검증한 것은 아닙니다. [조회 전용 예제](../../../examples/data-sources/iwinv_db_instance_products/main.tf)를 참고하세요.

출처: [공식 DBMS 상품 API](https://iwinv-dbms.readme.io/reference/클라우드-dbms-상품-조회).
