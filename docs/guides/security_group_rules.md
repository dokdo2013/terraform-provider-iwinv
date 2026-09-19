---
page_title: "Security group rule lifecycle and recovery"
subcategory: ""
description: |-
  Ownership, updates, replacement, import and failure recovery for security group rules.
---

# Security-group rule lifecycle and recovery

[한국어](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/docs/ko/guides/security_group_rules.md) · [Ingress](../resources/security_group_ingress_rule.md) · [Egress](../resources/security_group_egress_rule.md)

Each resource owns one exact numeric rule ID within one exact parent group. It never adopts a matching tuple or name.
The parent resource manages group attributes only; independent rules are the sole Terraform writers of rule attributes.
Use references to establish dependencies and keep each rule in one state. Avoid manual deletion of a group that still has managed rules.

## Updates and replacements

Protocol (`tcp`/`udp`), port range, IPv4 CIDR, name and nonempty description updates preserve the rule ID.
The API can update direction, so an observed direction change is reflected in state and explicitly repaired to the direction selected by the resource type.
`direction` is computed; select the ingress or egress resource instead of setting it directly.

An empty/omitted description on create is returned as API null and represented as an empty Terraform string.
Empty and null description updates preserve existing text, so clearing an existing description requires destroy and recreate.
Changing `security_group_id` also requires replacement. An unknown new description with a nonempty prior value conservatively plans replacement.
These replacements can create a filtering gap. Review them before applying and use `prevent_destroy` where a gap is unacceptable.
Do not assume `create_before_destroy` will work: the API rejects exact duplicate rules, and the provider does not adopt an existing rule after that error.
Creating another rule is never an automatic retry of a failed request.

The API returned title/content verbatim in rule tests; do not URL-encode or HTML-decode these fields.
Group descriptions use a different contract. Host bits in IPv4 CIDR text are also preserved, without inferring packet behavior.
Single ports and equal port ranges map to equal `from_port`/`to_port` values.
Only TCP/UDP with ports 1–65535 and IPv4 CIDRs are accepted. Port 0, bare-IP update and IPv6/ICMP inputs failed the documented experiments.
These facts do not verify data-plane enforcement or every account/zone variant.

## Read and absence

The API exposes a parent-scoped rule list, not an individual detail endpoint.
The provider validates the entire unpaginated array, count, exact integer IDs and fields before looking up an ID.
A live 53-rule experiment returned all 53 IDs with default queries and with `page_no`/`page_size=1`; those pagination arguments were ignored.
Unexpected pagination metadata, malformed rows, duplicates or API errors stop the read without discarding state.
This is evidence beyond a 50-row boundary, not proof that all possible future account quotas are unbounded.

A parent detail Read is performed first. The verified parent HTTP 200/empty-array/count-zero contract establishes parent absence.
If the parent exists, only a successful complete rule list lacking the exact rule ID establishes child absence.
The rules endpoint can return `CHECK_PARAM` after parent deletion; that error alone is never treated as absence.
Losing a parent makes a child unaddressable, but does not independently prove physical cascade deletion or billing closure.

## Failure recovery

Create stores a returned integer ID as an exact decimal string and saves the composite identity before reporting later response or read-back errors.
Terraform may mark that object tainted. Inspect the remote object and the replacement plan before the next apply.
If identity is unknown, reconcile the private request record and inventory before creating anything else.
Writes are single-attempt. API errors do not trigger blind POST/PUT/DELETE retries or same-name adoption.

Update failures retain prior state; refresh reconciles a write that may have reached the service.
Delete reads first, sends at most one DELETE and verifies exact rule absence before returning success.
A failed delete/read-back retains state. Parent and peer rules are not deleted by an individual rule resource.
No passwords, tokens or raw response bodies are copied into the public resource state or diagnostics.

## Evidence and execution

T058 covers both direction resources, stable-ID updates, full import comparison, persisted re-import with no-op plans for both directions,
direction drift repair, explicit replacement plans for clearing/parent changes, external deletion/recreation and dependency-ordered destroy.
Synthetic tests also cover failed-create ID cleanup, update/delete failure state, parent absence, invalid inputs, timeout-only updates and unknown description planning.
The live test journals only this run's new parent and child IDs outside Git and checks child absence before parent deletion.

```sh
TF_ACC=1 IWINV_LIVE_TERRAFORM_RULE_WRITE=1 \
  IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  go test -race ./internal/provider -run '^TestAccSecurity(Rules|EgressRule)$' -v -timeout 20m
```

Use environment credentials as in the [development guide](https://github.com/dokdo2013/terraform-provider-iwinv/blob/main/design/en/development.md). This opt-in is never enabled in CI.
Keep state, raw output and cleanup journals private. A failed or unidentifiable create remains a reconciliation task, not a reason to repeat the request.

Official sources: [list](https://iwinv.readme.io/reference/get_v1-security-groups-id-rules), [create](https://iwinv.readme.io/reference/post_v1-security-groups-id-rules), [update](https://iwinv.readme.io/reference/put_v1-security-groups-id-rules-rule-id), [delete](https://iwinv.readme.io/reference/delete_v1-security-groups-id-rules-rule-id).
