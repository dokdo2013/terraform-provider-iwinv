terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

data "iwinv_db_instance_products" "all" {}

data "iwinv_db_instance_products" "redis" {
  product_type = "STD"
  engine       = "redis"
}

# Review rows explicitly. IDs may be empty or shared by versions.
# A filtered version cannot be sent to the creation API as a version selector.
output "all_products" {
  value = data.iwinv_db_instance_products.all.products
}

output "filtered_products" {
  value = data.iwinv_db_instance_products.redis.products
}
