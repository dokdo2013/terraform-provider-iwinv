terraform {
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

data "iwinv_security_groups" "all" {}

output "security_groups" {
  value = data.iwinv_security_groups.all.groups
}
