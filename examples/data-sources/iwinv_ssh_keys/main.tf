terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = {
      source = "dokdo2013/iwinv"
    }
  }
}

provider "iwinv" {}

data "iwinv_ssh_keys" "available" {}

# Optional exact ID: select the intended key, not the first list entry.
variable "ssh_key_id" {
  type    = string
  default = null
}

data "iwinv_ssh_key" "selected" {
  count = var.ssh_key_id == null ? 0 : 1
  id    = var.ssh_key_id
}

output "available_keys" {
  value = data.iwinv_ssh_keys.available.keys
}

output "selected_keys" {
  value = data.iwinv_ssh_key.selected
}
