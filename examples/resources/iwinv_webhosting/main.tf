terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = {
      source = "dokdo2013/iwinv"
    }
  }
}

provider "iwinv" {}

variable "product_id" { type = string }
variable "server_id" { type = string }
variable "account_name" { type = string }

variable "ftp_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "database_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "iwinv_webhosting" "example" {
  product_id           = var.product_id
  server_id            = var.server_id
  account_name         = var.account_name
  name                 = "tf-example-hosting"
  description          = "Managed by Terraform"
  web_firewall_enabled = true
  custom_domains       = {}
  ftp_password_wo      = var.ftp_password
  database_password_wo = var.database_password
  password_wo_version  = 1

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

output "hosting_id" {
  value = iwinv_webhosting.example.id
}

output "default_domain" {
  value = iwinv_webhosting.example.default_domain
}
