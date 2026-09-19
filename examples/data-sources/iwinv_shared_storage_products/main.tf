terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

data "iwinv_shared_storage_products" "all" {}

# Review nonempty IDs and bounds; the first row is not a recommended choice.
# 비어 있지 않은 ID와 용량 범위를 검토하세요. 첫 행은 추천 상품이 아닙니다.
output "storage_products" {
  value = data.iwinv_shared_storage_products.all.products
}
