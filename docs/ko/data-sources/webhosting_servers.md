---
page_title: "iwinv_webhosting_servers Data Source - iwinv"
subcategory: "Hosting"
description: |-
  지정한 웹호스팅 상품의 정확한 서버 선택지를 조회합니다.
---

# iwinv_webhosting_servers (Data Source)

[English](../../data-sources/webhosting_servers.md) · [개발용 설치](../../../design/ko/development.md)

명시적으로 선택한 상품의 서버 선택지를 조회합니다. 서비스를 생성하지 않으며 현재 개발 Provider 전용입니다.

```hcl
variable "hosting_product_id" { type = string }
data "iwinv_webhosting_servers" "selected" {
  product_id = var.hosting_product_id
}
output "hosting_server_choices" {
  value = data.iwinv_webhosting_servers.selected.servers
}
```

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `product_id` | 필수 string | [상품 목록](webhosting_products.md)의 비어 있지 않은 정확한 ID. 필수 쿼리로 전달. |
| `ids` | 계산 list(string) | 양의 정수 서버 `idx`를 정확한 십진 문자열로 보존하고 사전순 정렬. |
| `servers` | 계산 list(object) | `id`, `charset`, `php_version`, `database`, `program` 문자열. `ids`와 같은 순서. |

2^53보다 큰 서버 ID도 정확한 문자열로 보존합니다. 사전순 정렬은 숫자 크기나 추천 순서와 다릅니다.
서버 ID는 생성 선택값이며 생성된 서비스의 `service_idx`와 다릅니다. 서비스 Read로 복구할 수 없습니다.
`database`, `program`은 빈 문자열일 수 있습니다. PHP 라벨 등은 반환값 그대로이며 버전을 추측하지 않습니다.

문자셋·PHP·DB 요구사항을 검토해 선택하세요. 카탈로그에 있다고 가용성, 임의 입력과의 호환성이나 생성 성공을 보장하지 않습니다.
자동으로 첫 행을 선택하지 마세요. [통합 예제](../../../examples/data-sources/iwinv_webhosting_catalogs/main.tf)는 검토할 선택지를 출력합니다.

배열 전체를 검증한 뒤 결과를 제공합니다. 빈 목록은 정상이나 잘못된 행/중복, 변경된 페이지·건수 메타데이터와 API 오류는 실패합니다.
부분 목록이나 HTTP 404를 정상적인 부재로 취급하지 않습니다. import·변경·원격 소유권은 없습니다.
T061에 실환경 읽기/무변경 plan과 합성 계약 검증을 기록합니다.

출처: [공식 상품 상세 조회](https://iwinv-hosting.readme.io/reference/상품-상세-조회).
