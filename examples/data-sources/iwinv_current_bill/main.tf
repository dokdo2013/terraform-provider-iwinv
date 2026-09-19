terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

data "iwinv_current_bill" "current" {}

# Sensitive outputs are still stored in state and saved plans.
# 민감 출력도 state와 저장된 plan에 남습니다.
output "estimated_price" {
  value     = data.iwinv_current_bill.current.price
  sensitive = true
}
