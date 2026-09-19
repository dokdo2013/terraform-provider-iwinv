terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = {
      source = "dokdo2013/iwinv"
    }
  }
}

provider "iwinv" {}

data "iwinv_images" "available" {}
data "iwinv_instance_types" "available" {}

# Optional exact IDs: choose deliberately from the catalogs, not by list position.
variable "image_id" {
  type    = string
  default = null
}
variable "instance_type_id" {
  type    = string
  default = null
}

data "iwinv_image" "selected" {
  count = var.image_id == null ? 0 : 1
  id    = var.image_id
}
data "iwinv_instance_type" "selected" {
  count = var.instance_type_id == null ? 0 : 1
  id    = var.instance_type_id
}

output "image_ids" {
  value = data.iwinv_images.available.ids
}
output "instance_type_ids" {
  value = data.iwinv_instance_types.available.ids
}
output "selected_images" {
  value = data.iwinv_image.selected
}
output "selected_instance_types" {
  value = data.iwinv_instance_type.selected
}
