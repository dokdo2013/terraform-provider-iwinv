terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

data "iwinv_bills" "selected" {
  # Example bill-date filters; omit for all API-visible bill summaries.
  # 청구일 필터 예제입니다. 생략하면 API에서 보이는 전체 요약을 읽습니다.
  start_date    = "2026-01-01"
  end_date      = "2026-01-31"
  minimum_price = 0
}

# Sensitive is redaction, not encryption or exclusion from state.
# sensitive는 가림 표시이며 암호화나 state 제외가 아닙니다.
output "bill_summaries" {
  value     = data.iwinv_bills.selected.bills
  sensitive = true
}
