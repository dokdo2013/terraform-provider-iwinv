---
page_title: "iwinv_shared_storage_products Data Source - iwinv"
subcategory: "Storage"
description: |-
  null 버전과 용량 범위를 보존하며 API NAS 상품을 조회합니다.
---

# iwinv_shared_storage_products (Data Source)

[English](../../data-sources/shared_storage_products.md) · [개발 버전 설치](../../../design/ko/development.md)

스토리지를 생성하지 않고 API NAS 전체 상품을 조회합니다. 개발 Provider이며 Registry 릴리스는 아직 없습니다.
이 엔드포인트에 문서화된 필터가 없으므로 사용자가 설정할 속성도 없습니다.

```hcl
data "iwinv_shared_storage_products" "all" {}

output "storage_products" {
  value = data.iwinv_shared_storage_products.all.products
}
```

`products`는 상품 ID·이름 순으로 사전 정렬한 계산 list(object)입니다. 빈 ID가 먼저 나옵니다.
각 행은 다음 속성을 포함합니다.

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `product_id` | string | 정확한 생성 ID. 빈 ID도 보존하지만 서비스 생성에는 사용할 수 없습니다. |
| `name` | string | 관측한 상품 이름 원문. |
| `status` | string | 관측한 제공 상태. |
| `version` | nullable string | 관측 버전. null과 빈 문자열을 구분하며 독립적인 생성 버전 선택자가 아닙니다. |
| `minimum_size_gb` | int64 | 문서상 GB 단위 최소 용량. 준비 중인 행에는 0일 수 있습니다. |
| `maximum_size_gb` | int64 | 문서상 GB 단위 최대 용량. resize 지원을 뜻하지 않습니다. |

[`iwinv_shared_storage`](../resources/shared_storage.md)에 사용하기 전 비어 있지 않은 ID와 용량 범위를 검토하세요.
첫 행을 기본 상품으로 자동 선택하지 마세요. available과 유효한 범위만으로 실제 계정의 생성 자격을 입증하지 않습니다.
현재 리소스는 용량이 바뀌면 스토리지를 교체하며 카탈로그가 제자리 증설·이전을 의미하지 않습니다.
가격과 임의 응답 필드는 제외합니다. import나 원격 소유 범위는 없습니다.

빈 배열은 유효합니다. ID 누락·null·문자열 아닌 값, 명시적인 null 외 버전 누락·잘못된 타입,
정수 용량 누락·잘못된 타입·음수·역전, 비어 있지 않은 ID 중복·빈 ID 이름 중복,
페이지/건수 메타데이터 변경과 API 오류는 전체 Read를 실패 처리합니다.
이름이 다른 준비 중 상품은 별도 행으로 보존하고 API 목록 순서 변경은 state 변경을 만들지 않습니다.

T070은 합성 오류·빈 목록·정렬 검사와 실환경 상품 조회 후 무변경 plan을 다룹니다.
실측에서는 100–2000 GB의 available api_nas와 빈 ID·null 버전·용량 0의 준비 중 상품을 확인했습니다.
다른 상품·파일 접근·가격·과금은 별도 검증 대상입니다.

[읽기 전용 예제](../../../examples/data-sources/iwinv_shared_storage_products/main.tf) ·
[검증 근거](../../../design/ko/contract-progress.md)

출처: [공식 API NAS 상품 조회](https://iwinv-api-nas.readme.io/reference/공유-스토리지-상품-조회).
