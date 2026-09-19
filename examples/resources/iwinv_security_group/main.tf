terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = {
      source = "dokdo2013/iwinv"
    }
  }
}

provider "iwinv" {}

resource "iwinv_security_group" "example" {
  name        = "tf-example-group"
  description = "Managed by Terraform"
  allow_icmp  = false

  timeouts {
    create = "5m"
    read   = "1m"
    update = "5m"
    delete = "5m"
  }
}

output "security_group_id" {
  value = iwinv_security_group.example.id
}
