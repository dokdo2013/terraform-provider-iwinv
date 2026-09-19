terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}
provider "iwinv" {}

data "iwinv_content_cache_products" "all" {}
data "iwinv_content_cache_products" "shared" {
  product_type = "SHARE"
}
output "all_products" {
  value = data.iwinv_content_cache_products.all.products
}
output "shared_products" {
  value = data.iwinv_content_cache_products.shared.products
}
