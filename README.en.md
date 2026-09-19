# Terraform Provider for iwinv

English · [한국어](README.md)

An independent community provider project for Terraform users familiar with the AWS provider.

**Status: development provider and five zone/image/instance-type data sources implemented and live-tested. No Registry release yet.**
Managed resources remain unimplemented. Resource examples in design documents are proposals; see the [development guide](design/en/development.md) for runnable scope.
This project is not an official SMILESERV/iwinv product or support channel.

## Goals

- Track and progressively support every resource and operation exposed by official APIs and CLI.
- Provide lifecycle management, existing-resource import, drift detection, and predictable plans.
- Adopt familiar AWS-style workflows without inventing unsupported iwinv capabilities.
- Maintain Korean and English documentation, examples, and troubleshooting together.

## Design documents

| Document | Contents |
| --- | --- |
| [User experience and architecture](design/en/architecture.md) | AWS mapping, schemas, state, authentication, errors |
| [API contracts and limitations](design/en/api-contract.md) | Evidence, inconsistencies, live verification gaps |
| [Full capability coverage](design/en/coverage.md) | Resource/Data/Action/Ephemeral classification |
| [Verification plan](design/en/verification.md) | Acceptance criteria and execution checklist |
| [Roadmap](design/en/roadmap.md) | Dependencies and completion criteria |
| [Bilingual documentation policy](design/en/documentation.md) | Beginner guides, terminology, translation parity |
| [Evidence and sources](design/sources.md) | Observation date and official references |

## Progress

- [x] Initial documentation research and design repository
- [x] 66 HTTP operations identified across 75 public control-plane documentation pages
- [x] Command and URL references collected from 21 CLI and messaging pages
- [ ] Verify authenticated API responses and behavior
- [x] Go read/write client and synthetic contract tests
- [x] Provider skeleton and authenticated read-only zone/image/instance-type acceptance
- [ ] Managed resource implementation
- [ ] Resource acceptance tests and signed releases
- [ ] Publish to Terraform Registry

These counts describe research scope, not implementation coverage. Service APIs, S3 compatibility,
CLI subcommand options, and authenticated MCP tools still require discovery.
CLI v0.2.2 subcommands and flags have also been inspected. Verified capabilities are recorded separately in the [implementation ledger](design/inventory/implementation.json).

Evidence and commands: [P1 contract progress](design/en/contract-progress.md).

## Contributing and validation

Read [CONTRIBUTING.md](CONTRIBUTING.md). Documentation checks require Python 3.10 or newer:

```sh
python3 scripts/check_docs.py
```

Manually refresh public documentation metadata and review the diff with:

```sh
python3 scripts/discover_api.py
python3 scripts/discover_surfaces.py
```

These commands neither authenticate to accounts nor create resources.
License: [MPL-2.0](LICENSE). This independent project contains no existing organizational code, configuration, or account data.
