---
page_title: "iwinv_current_bill Data Source - iwinv"
subcategory: "Billing"
description: |-
  부가세 제외 현재 예상 청구를 민감 출력으로 조회합니다.
---

# iwinv_current_bill (Data Source)

[English](../../data-sources/current_bill.md) · [개발 버전 설치](../../../design/ko/development.md)

이번 달 현재까지의 예상 금액을 조회하는 개발 Data Source입니다. 결제하거나 청구서를 만들지 않습니다.
예상 금액은 refresh마다 바뀔 수 있으며 확정 청구나 삭제한 인프라의 과금 종료 증거가 아닙니다.

```hcl
data "iwinv_current_bill" "current" {}

output "estimated_price" {
  value     = data.iwinv_current_bill.current.price
  sensitive = true
}
```

입력 인자는 없으며 계산 속성 5개 모두 민감 값입니다.

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `bill_id` | string | 관측한 현재 청구 ID. `BILL-live`를 검증했습니다. |
| `start_date` | string | 관측한 시작 날짜 원문. |
| `end_date` | string | 관측한 종료 날짜 원문. |
| `price` | int64 | 반환 통화의 부가세 제외 예상 금액. 단위 배율을 적용하지 않습니다. |
| `currency` | string | 관측 통화 코드. KRW를 검증했습니다. |

API는 배열을 반환하며 검증된 행이 정확히 하나여야 합니다. 0개 또는 여러 개면 임의 선택하지 않고 진단을 반환합니다.
HTTP·권한 오류, 잘못된 데이터, count 불일치와 예상하지 않은 페이지 메타데이터도 조회를 실패시킵니다.
2^53보다 큰 값도 정확한 정수로 보존하며 부가세 계산이나 통화 환산을 하지 않습니다.
청구 시간대와 월초 경계는 미검증이므로 날짜 문자열을 UTC로 바꾸지 않습니다.

**민감 값도 Terraform state와 저장된 plan에 남습니다.** 일반 사람이 읽는 출력에서는 가리지만
`terraform show -json` 같은 명령은 원래 값을 보여줍니다. state·plan·로그를 보호하세요. `sensitive`는 암호화나 비저장이 아닙니다.
루트 output에는 `sensitive = true`를 명시해야 합니다. 결제수단·영수증·세금계산서 링크는 노출하지 않습니다.

import나 원격 소유 범위는 없습니다. T072는 Core 민감 표시, 정확한 금액, 결과 개수·오류 처리와 읽기 전용 실환경 사용을 검증합니다.
금액이 안정적인 구간에서는 무변경 plan을 관측할 수 있지만 미래 refresh의 금액 불변을 보장하지 않습니다.
별도 `iwinv_bill` 상세 타입은 현재 테스트 접근이 거절되어 구현하지 않았습니다.

[예제](../../../examples/data-sources/iwinv_current_bill/main.tf) · [계약과 한계](../../../design/ko/billing-contract.md) ·
[청구 목록](bills.md)

출처: [공식 현재 예상 청구 API](https://iwinv-common.readme.io/reference/get_new-endpoint).
