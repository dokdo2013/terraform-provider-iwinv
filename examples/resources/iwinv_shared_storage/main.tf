terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

variable "product_id" {
  type        = string
  description = "Reviewed available NAS creation product ID / 검토한 NAS 생성 상품 ID"
}

variable "share_name" {
  type        = string
  description = "Fresh 6–20 ASCII alphanumeric share name / 새 공유 이름"
}

resource "iwinv_shared_storage" "example" {
  product_id  = var.product_id
  share_name  = var.share_name
  name        = "tf-example-storage"
  description = "Terraform example / 공유 스토리지 예제"
  size_gb     = 100
  # Replace documentation addresses with reviewed client IPv4 hosts.
  # 문서용 IP를 검토한 클라이언트 IPv4로 바꾸세요.
  allowed_ips = {
    "192.0.2.10" = "RW"
    "192.0.2.11" = "RO"
  }

  # Capacity changes replace storage; this does not migrate files.
  # 용량 변경은 스토리지를 교체하며 파일을 이전하지 않습니다.
  lifecycle {
    prevent_destroy = true
  }
}
