terraform {
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

data "iwinv_block_storage_types" "all" {}

data "iwinv_block_storage_types" "ssd" {
  type = "ssd"
}

output "block_storage_types" {
  value = data.iwinv_block_storage_types.all.types
}
