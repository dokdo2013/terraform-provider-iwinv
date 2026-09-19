terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = {
      source = "dokdo2013/iwinv"
    }
  }
}
provider "iwinv" {}

variable "product_id" {
  type        = string
  description = "Exact product ID chosen after reviewing the product catalog."
}

data "iwinv_webhosting_products" "shared" {
  type = "SHARE"
}

data "iwinv_webhosting_servers" "selected" {
  product_id = var.product_id
}

output "products" {
  value = data.iwinv_webhosting_products.shared.products
}

output "server_choices" {
  value = data.iwinv_webhosting_servers.selected.servers
}
