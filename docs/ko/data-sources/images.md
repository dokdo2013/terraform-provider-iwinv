---
page_title: "iwinv_images Data Source - iwinv"
subcategory: "Compute"
description: |-
  API에 보이는 이미지 ID 전체를 조회합니다. 결과를 확인한 뒤 의도한 이미지를 직접 선택하세요.
---

# iwinv_images

[English](../../data-sources/images.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_images-data-source)

API에 보이는 이미지 ID 전체를 조회합니다. 결과를 확인한 뒤 의도한 이미지를 직접 선택하세요.

개발 버전이며 Registry 릴리스는 아직 없습니다. [Provider 설정](../index.md)을 먼저 확인하세요.

## 사용 예제

```hcl
data "iwinv_images" "available" {}

output "image_ids" {
  value = data.iwinv_images.available.ids
}
```

## 입력 인자

서비스별 입력 인자는 없습니다. 이름 필터·most_recent·이미지 자동 선택은 구현하지 않았습니다.

## 조회 속성

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `ids` | `list(string)` | 전체 페이지의 정확한 이미지 ID를 사전순으로 정렬합니다. |

모든 속성은 계산 속성이므로 설정에 값을 지정하지 않습니다.

## 동작과 제약

페이지당 10개를 읽고 짧은 페이지에서 종료합니다. 마지막 페이지가 꽉 차면 다음 페이지도 조회합니다. 페이지·count 불일치, 중복 ID, 중간 페이지 오류가 있으면 전체 조회를 실패 처리합니다. 정상 빈 카탈로그는 빈 목록입니다. 최대 1,000페이지로 제한하며 snapshot 토큰이 없어 조회 중 변경을 완전히 배제할 수는 없습니다. 정렬 순서는 최신순이나 추천순이 아닙니다. 지원하는 상세 필드는 정확한 ID의 [image](image.md)로 조회하세요. 공개 이미지 상세만 실환경 검증했으며 비공개 이미지 응답 변형·추가 메타데이터는 미검증입니다. 조회됐다고 존·상품 호환성이나 생성 권한을 보장하지 않습니다.

조회 전용이며 import는 해당하지 않습니다. API 오류를 빈 결과로 바꾸지 않습니다.

[Example](../../../examples/data-sources/iwinv_catalogs/main.tf) · [Evidence](../../../design/ko/contract-progress.md)

[공식 API](https://iwinv.readme.io/reference/getv1images)
