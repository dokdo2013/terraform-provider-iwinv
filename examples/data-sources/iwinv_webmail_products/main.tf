terraform {
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

data "iwinv_webmail_products" "all" {}

output "webmail_products" {
  value = data.iwinv_webmail_products.all.products
}
