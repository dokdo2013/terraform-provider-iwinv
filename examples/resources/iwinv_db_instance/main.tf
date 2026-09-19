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
  description = "Exact nonempty DBMS creation product ID; review catalog version ambiguity."
}
variable "account_name" {
  type        = string
  description = "Fresh initial account using 6–12 ASCII letters."
}
variable "allowed_ips" {
  type        = set(string)
  description = "Nonempty authoritative set of permitted IPv4 host addresses."
}

resource "iwinv_db_instance" "example" {
  product_id   = var.product_id
  account_name = var.account_name
  name         = "tf-example-dbms"
  description  = "Managed by Terraform"
  allowed_ips  = var.allowed_ips

  lifecycle {
    prevent_destroy = true
  }

  timeouts {
    create = "5m"
    read   = "1m"
    update = "5m"
    delete = "5m"
  }
}

output "dbms_service_id" {
  value = iwinv_db_instance.example.id
}
output "dbms_address" {
  value = iwinv_db_instance.example.address
}
