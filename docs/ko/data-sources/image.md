---
page_title: "iwinv_image Data Source - iwinv"
subcategory: "Compute"
description: |-
  정확한 이미지 ID 하나의 지원 메타데이터를 읽습니다. 이미지를 생성·import·소유하지 않습니다.
---

# iwinv_image

[English](../../data-sources/image.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_image-data-source)

정확한 이미지 ID 하나의 지원 메타데이터를 읽습니다. 이미지를 생성·import·소유하지 않습니다.

개발 버전이며 Registry 릴리스는 아직 없습니다. [Provider 설정](../index.md)을 먼저 확인하세요.

## 사용 예제

```hcl
variable "image_id" {
  type = string
}

data "iwinv_image" "selected" {
  id = var.image_id
}

output "image_visibility" {
  value = data.iwinv_image.selected.visibility
}
```

## 입력 인자

`id` (필수 string): [images](images.md)에서 선택한 정확한 이미지 ID입니다. Read 시점에 알려진 비어 있지 않은 값이어야 합니다. 단일 ID의 허용 형식은 `[A-Za-z0-9_-][A-Za-z0-9_.-]*`이며 슬래시·공백·query 조각은 API 요청 전에 거절합니다.

## 조회 속성

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `id` | `string` | 설정한 정확한 ID이며 응답과 일치하는지 검증합니다. |
| `visibility` | `string` | API의 visibility 원문입니다. |
| `image_type` | `string` | API의 이미지 유형 원문입니다. |

`id`는 필수 입력이며 나머지는 계산 속성입니다.

## 동작과 제약

응답에 요청 ID와 일치하는 행이 정확히 하나 있고 상세 필드가 비어 있지 않아야 합니다. 결과 없음·복수 행·ID 불일치는 오류이며 첫 행이나 이름으로 대체하지 않습니다. 이미지 비밀번호나 최신 이미지 자동 선택을 제공하지 않습니다. 한정된 공개 이미지 계약에는 실환경 조회·무변경 plan 근거가 있습니다. 비공개 이미지 응답 변형, 이미지 생성·삭제, 아키텍처·OS 상세 사양, 인스턴스 호환성은 이 Data Source의 검증 범위가 아닙니다.

조회 전용이며 import는 해당하지 않습니다. API 오류를 빈 결과로 바꾸지 않습니다.

[Example](../../../examples/data-sources/iwinv_catalogs/main.tf) · [Evidence](../../../design/ko/contract-progress.md)

[공식 API](https://iwinv.readme.io/reference/getv1imagesimageid)
