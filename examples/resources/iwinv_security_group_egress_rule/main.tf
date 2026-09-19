terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}

provider "iwinv" {}

resource "iwinv_security_group" "example" {
  name        = "tf-egress-example"
  description = "Managed by Terraform"
}

resource "iwinv_security_group_egress_rule" "example" {
  security_group_id = iwinv_security_group.example.id
  name              = "example-egress"
  description       = "Example rule"
  ip_protocol       = "udp"
  from_port         = 53
  to_port           = 53
  cidr_ipv4         = "192.0.2.53/32"
}

output "rule_identity" {
  value = iwinv_security_group_egress_rule.example.id
}
