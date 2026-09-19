terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

variable "product_id" {
  type        = string
  description = "Reviewed nonempty cache product ID."
}
variable "account_name" {
  type        = string
  description = "Fresh 6–12 ASCII alphanumeric account; do not reuse a deleted account for 24 hours."
}
variable "ftp_password" {
  type      = string
  sensitive = true
  ephemeral = true
}
variable "allowed_referrers" {
  type        = set(string)
  default     = []
  description = "Complete desired lowercase DNS hostname set; clearing a nonempty set replaces the service."
}

resource "iwinv_content_cache" "example" {
  product_id          = var.product_id
  account_name        = var.account_name
  name                = "tf-example-cache"
  allowed_referrers   = var.allowed_referrers
  ftp_password_wo     = var.ftp_password
  password_wo_version = 1

  lifecycle { prevent_destroy = true }
}
