---
page_title: "iwinv_bills Data Source - iwinv"
subcategory: "Billing"
description: |-
  날짜·금액 필터와 민감 출력을 사용해 전체 청구 요약을 조회합니다.
---

# iwinv_bills (Data Source)

[English](../../data-sources/bills.md) · [개발 버전 설치](../../../design/ko/development.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_bills-data-source)

청구 기록을 변경하지 않고 전체 요약을 조회합니다. Registry 릴리스는 아직 없습니다.
목록 API를 사용하며 접근할 수 없는 상세 API의 중첩 group/item을 대신하지 않습니다.

```hcl
data "iwinv_bills" "selected" {
  start_date    = "2026-01-01"
  end_date      = "2026-01-31"
  minimum_price = 0
}

output "bill_summaries" {
  value     = data.iwinv_bills.selected.bills
  sensitive = true
}
```

날짜는 예시 필터입니다. 네 필터를 모두 생략하면 API에서 보이는 청구 요약 전체를 조회합니다.

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `start_date` | 선택 string | YYYY-MM-DD의 **청구일** 하한, 경계 포함. 사용 시작일이 아닙니다. |
| `end_date` | 선택 string | YYYY-MM-DD의 청구일 상한, 경계 포함. |
| `minimum_price` | 선택 sensitive int64 | 부가세 제외 `price` 하한, 경계 포함. |
| `maximum_price` | 선택 sensitive int64 | 부가세 제외 `price` 상한, 경계 포함. |
| `bills` | 계산 sensitive list(object) | 정확한 청구 ID 순으로 정렬한 전체 행. |

생략/null 필터는 제한이 없습니다. 빈 날짜·잘못된 달력 날짜·뒤집힌 범위는 요청 전에 거절합니다.
0·음수 금액은 그대로 전송하고 unknown 필터는 Read 전에 해석되어야 하며 몰래 계정 전체 조회로 바꾸지 않습니다.
응답이 요청한 필터에 맞는지도 검사합니다. 통화 환산이나 최소 통화 단위 배율을 적용하지 않습니다.

각 청구에는 string `bill_id`, `usage_start`, `usage_end`, `bill_date`, `type`, `payment_status`, `name`, `currency`, `date_paid`와
정확한 int64 `price`, `vat`, `payment_price`가 있습니다. price는 부가세 제외 금액이고 부가세·부가세 포함 결제금액은 계산하지 않고 관측합니다.
합계 일치를 강제하지 않습니다. 빈 결제일을 포함한 이름·날짜 원문을 유지하고 시간대를 추정하지 않습니다.
KRW는 실환경 근거가 있지만 다른 통화·환불/크레딧·미납 변형은 미검증입니다.
결제수단 정보·영수증/세금계산서 링크는 완전히 제외하며 raw JSON 우회 출력은 없습니다.

**민감 표시를 해도 전체 재무 결과는 state와 저장된 plan에 남습니다.** 루트 출력에는 `sensitive = true`가 필요하고
JSON 출력은 원래 값을 드러낼 수 있습니다. 산출물·로그를 보호하세요. 민감 표시는 가림 처리이며 암호화·비저장이 아닙니다.

페이지별 count와 정수/정규 정수 문자열 페이지 메타데이터를 검증하고 한도 내에서 전체 페이지를 순회합니다.
중복 ID·메타데이터 변경·잘못된 행·중간 오류는 부분 결과 없이 실패합니다.
첫 페이지의 정확한 HTTP 400 `EMPTY_SET`은 빈 목록으로 처리하고 성공한 빈 배열도 수용합니다.
일반 400/404·권한 오류·중간 페이지 EMPTY_SET은 오류입니다. snapshot token이 없어 동시 변경 중 일부 누락은 발견하지 못할 수 있습니다.
자동 재시도·import·원격 리소스 소유 범위는 없습니다.

T072는 안정 정렬·무변경 plan, 2^53 초과 금액, plan/state 민감 표시, 표시 없는 출력 거절, 결제정보 제외,
unknown·0·음수 필터와 읽기 전용 실환경 acceptance를 다룹니다. 요약 조회이며 상세·결제 지원이 아닙니다.

[예제](../../../examples/data-sources/iwinv_bills/main.tf) · [계약과 근거](../../../design/ko/billing-contract.md) ·
[현재 예상 청구](current_bill.md)

출처: [공식 청구 목록 API](https://iwinv-common.readme.io/reference/get_new-endpoint-1).
