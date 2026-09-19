terraform {
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

variable "security_group_id" {
  type        = string
  description = "Exact existing iwinv FIREWALL ID."
}

data "iwinv_security_group" "selected" {
  id = var.security_group_id
}

output "security_group_name" {
  value = data.iwinv_security_group.selected.name
}
