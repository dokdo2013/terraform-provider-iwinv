# Contributor and agent instructions

This is an independent community project owned by `dokdo2013`.

- Start with `README.md`, `design/ko/architecture.md`, and `design/ko/verification.md` (English equivalents live in `design/en/`).
- This repository has a development provider, nine live-tested zone/image/instance-type/SSH-key/hosting-catalog data sources, four live-tested security-group/rule/webhosting resources, and contract tooling. Attachments and other managed resources remain unimplemented. No Registry release exists yet. Keep claims aligned with the implementation ledger and evidence.
- Keep API facts, design proposals, and live observations distinct. Cite public primary sources and record unresolved API contracts.
- Maintain Korean and English documentation together. Identifiers and executable configuration remain English.
- Follow Terraform Plugin Framework conventions. Call supported service APIs directly; do not automate a browser or shell out to iwinv CLI from the provider.
- Never fabricate AWS capabilities such as tags, IAM, ARNs, or regions that the iwinv API does not expose.
- Every new API capability needs an inventory entry, ownership and lifecycle design, import decision, and verification evidence.
- Do not put credentials, account responses, real infrastructure identifiers, state, plans, or private organizational material in this public repository.
- Cloud-changing tests require a specifically scoped test account/environment and cost/cleanup controls. Repository work is not authorization to mutate existing infrastructure.
- The default branch is `main`. Validate with `python3 scripts/check_docs.py`, `go test -race ./...`, and `go vet ./...` before committing. Commit and push authorized changes; verify the remote commit and CI.
- Do not change Git author configuration or overwrite unrelated work.
