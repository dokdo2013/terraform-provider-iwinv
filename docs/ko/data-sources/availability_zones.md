---
page_title: "iwinv_availability_zones Data Source - iwinv"
subcategory: "Compute"
description: |-
  API에 보이는 존 목록을 조회합니다. 존을 관리하거나 서버를 생성하지 않습니다.
---

# iwinv_availability_zones

[English](../../data-sources/availability_zones.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_availability_zones-data-source)

API에 보이는 존 목록을 조회합니다. 존을 관리하거나 서버를 생성하지 않습니다.

개발 버전이며 Registry 릴리스는 아직 없습니다. [Provider 설정](../index.md)을 먼저 확인하세요.

## 사용 예제

```hcl
data "iwinv_availability_zones" "available" {}

output "zone_ids" {
  value = data.iwinv_availability_zones.available.zone_ids
}
```

## 입력 인자

서비스별 입력 인자는 없습니다. region·상태 필터를 제공하지 않습니다.

## 조회 속성

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `zone_ids` | `list(string)` | 정확한 API ID를 사전순으로 정렬합니다. |
| `names` | `list(string)` | zone_ids와 같은 순서의 표시 이름입니다. |
| `zones` | `list(object)` | 문자열 id·name·status를 가진 행이며 zone_ids와 같은 순서입니다. |

모든 속성은 계산 속성이므로 설정에 값을 지정하지 않습니다.

## 동작과 제약

ID와 상태 문자열을 원문 그대로 유지합니다. 모든 행·응답 count·ID 중복 여부를 검사합니다. 정상 빈 배열은 빈 카탈로그이며 잘못된 응답이나 API 오류는 조회 실패입니다. 표시 이름과 상태를 AWS region이나 가용성 보장으로 변환하지 않습니다. 존이 보여도 콘솔의 모든 서버가 이 API에 노출된다는 뜻은 아닙니다. 인스턴스 생성 리소스는 아직 구현하지 않았습니다.

조회 전용이며 import는 해당하지 않습니다. API 오류를 빈 결과로 바꾸지 않습니다.

[Example](../../../examples/data-sources/iwinv_availability_zones/main.tf) · [Evidence](../../../design/ko/contract-progress.md)

[공식 API](https://iwinv.readme.io/reference/getv1zones)
