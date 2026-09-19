terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

resource "iwinv_security_group" "example" {
  name        = "tf-ingress-example"
  description = "Managed by Terraform"
}

resource "iwinv_security_group_ingress_rule" "example" {
  security_group_id = iwinv_security_group.example.id
  name              = "example-ingress"
  description       = "Example rule"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = "192.0.2.0/24"
}

output "rule_identity" {
  value = iwinv_security_group_ingress_rule.example.id
}
