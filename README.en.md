# Terraform Provider for iwinv

English · [한국어](README.md)

An independent community provider project for Terraform users familiar with the AWS provider.

**Status: eighteen data sources and seven security-group/rule/webhosting/DBMS/cache/NAS resources implemented and live-tested. No Registry release yet.**
See the [development guide](design/en/development.md) and [security-group guide](docs/resources/security_group.md) for runnable scope. Other proposed resources, including instances and attachments, are not yet available.
This project is not an official SMILESERV/iwinv product or support channel.

## Goals

- Track and progressively support every resource and operation exposed by official APIs and CLI.
- Provide lifecycle management, existing-resource import, drift detection, and predictable plans.
- Adopt familiar AWS-style workflows without inventing unsupported iwinv capabilities.
- Maintain Korean and English documentation, examples, and troubleshooting together.

## Design documents

| Document | Contents |
| --- | --- |
| [Provider getting started](docs/index.md) | Development installation, credentials, feature selection and troubleshooting |
| [User experience and architecture](design/en/architecture.md) | AWS mapping, schemas, state, authentication, errors |
| [Hosting lifecycle design](design/en/webhosting-lifecycle.md) | Account reuse restriction, password/import/replacement policies; SHARE PHP 8.4 live-tested |
| [Cache lifecycle decisions](design/en/cache-lifecycle.md) | Nested password input, referrer-set replacement, busy errors and cleanup; Terraform import/replacement/write-only verification |
| [Cache data API preparation](design/en/cache-data-api.md) | Product-to-manager mapping, separate key authentication and image/folder verification plan |
| [NAS lifecycle decisions](design/en/nas-lifecycle.md) | Whole RO/RW map, import without share history, capacity replacement and cleanup |
| [DBMS lifecycle decisions](design/en/dbms-lifecycle.md) | Authoritative allowlists, account history, product ambiguity, import and recovery |
| [Billing read contracts](design/en/billing-contract.md) | Exact money, dates, pagination and sensitive state; current/list data sources and detail access gap |
| [MCP audit](design/en/mcp-audit.md) | OAuth consent/registration cleanup gates and offline tool-inventory validation |
| [Release preparation](design/en/release-readiness.md) | Seven-target packaging, checksums, isolated installation and remaining signing/Registry gates |
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
- [x] Provider skeleton and authenticated read-only zone/image/instance-type/SSH-key acceptance
- [x] Security-group attributes: create/update/import/drift/destroy verification
- [x] Ingress/egress rules: import, drift, replacement and dependency-ordered deletion
- [x] Hosting product/server catalog reads and no-change plans
- [x] Hosting fresh-account replacement/import/write-only secret exclusion/destroy (SHARE PHP 8.4)
- [x] DBMS product reads across all documented filters, preserving empty and version-shared IDs
- [x] DBMS allowlist replacement/import/drift/fresh-account replacement/cleanup (STD Redis)
- [x] Cache referrer updates, fresh-account clear/replacement, import, secret exclusion and cleanup (`cache_lite`)
- [x] Cache product unfiltered/SHARE/SINGLE reads, null-ID preservation and no-change plans
- [x] [NAS resource](docs/resources/shared_storage.md) readiness, permission updates, import, capacity replacement and cleanup
- [x] [NAS product catalog](docs/data-sources/shared_storage_products.md) with empty IDs, nullable versions, GB bounds and no-change plan
- [x] [Current estimate](docs/data-sources/current_bill.md) and [bill list](docs/data-sources/bills.md) with exact money, sensitive plan/state and live reads
- [ ] Remaining managed resources
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
