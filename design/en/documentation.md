# Korean and English documentation

[한국어](../ko/documentation.md) · [Index](../../README.en.md) · Revision: 1

Korean and English are equal baseline languages. Root README is Korean with an immediately visible English link.
Design files use matching paths under `design/ko` and `design/en`; each change updates both in the same PR.
Schema names and API identifiers stay English. Prefer plain Korean explanations over untranslated jargon.

## Planned user journeys

1. Before first use: support status, API enablement, account/zone coverage, costs and authentication families.
2. Setup on macOS/Windows/Linux, version constraints, provider aliases and environment credentials.
3. Existing-resource-first adoption: inventory, ID lookup, import, matching configuration, no-change plan.
4. Disposable first server: deterministic image/spec selection, plan, apply, wait, inspect, teardown and billing check.
5. Security groups, storage and additional services with explicit ownership and data-retention diagrams.
6. Team use: protected remote state, lock file, CI egress, secrets, quota sharing, review and upgrades.
7. Troubleshooting by stable diagnostic code: IP deny, clock skew, 429, invisible zone, ambiguous create, import drift.
8. Advanced Actions/Ephemeral usage, explicit disruption/duplicate-delivery consequences and version requirements.

Every feature page includes availability/version, prerequisites, minimal example, argument/attribute tables,
update vs replacement, import syntax, timeout/defaults, permissions, billing/data-loss implications, limitations,
troubleshooting and official API links. Translate behavior, not just titles.

## Source and generation layout

Current capability pages are maintained in `docs/` and `docs/ko/`, with direct links between languages; do not assume the Registry supports a locale router. The [complete schema reference](../../docs/guides/schema_reference.md) is generated in both languages from the actual provider binary. It covers provider configuration, resources, data sources, nested attributes, timeout blocks and input/sensitivity/write-only flags. Curated pages retain behavioral and lifecycle explanations that schema JSON cannot express. Official `tfplugindocs` format validation is now included as described below; the official body preview was checked separately, while published Registry navigation remains unverified.

All 52 provider/capability HCL snippets currently pass format and validation with a common provider configuration added where needed. Korean and English executable snippets are identical. Each page links to the same complete example directory. Design proposals elsewhere remain explicitly non-runnable. The historical `scripts/check_intro_docs.py` name now covers all provider pages, compares runtime registration with the capability ledger, and checks both generated schema references. It uses an isolated CLI configuration without account credentials, init, plan or apply.

To regenerate after a schema change, build the provider and run the following, then review the diff and repeat without the write flag:

```sh
go build -o bin/terraform-provider-iwinv .
python3 scripts/check_intro_docs.py --provider-dir bin --terraform /absolute/path/to/terraform --write-schema-reference
python3 scripts/check_intro_docs.py --provider-dir bin --terraform /absolute/path/to/terraform
```

Use an actual Terraform executable, not a home-dependent version-manager wrapper. CI checks without regenerating on Terraform 1.14.0 and 1.14.2. T080 covers this structural/example verification. The [feature-guide behavior review](guide-review.md) records a scoped bilingual review of the 25 registered capabilities and its corrections. T052 still requires narrative review of the introduction/shared guides and published Registry navigation checks; generated tables do not prove default, validator, plan-modifier or live API behavior. A bilingual documentation site and Registry publication remain release work.

### Official Registry format validation

CI also supplies `--tfplugindocs bin/tfplugindocs`, installed with `GOBIN="$PWD/bin" go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0`. This runs `validate`, never `generate`, against the same freshly exported runtime schema and the curated pages. Local reproduction:

```sh
GOBIN="$PWD/bin" go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0
python3 scripts/check_intro_docs.py --provider-dir bin --terraform /absolute/path/to/terraform --tfplugindocs bin/tfplugindocs
python3 scripts/test_doc_failures.py --provider-dir bin --terraform /absolute/path/to/terraform --tfplugindocs bin/tfplugindocs
```

Version 0.25.0 looks up the JSON provider schema by short name or a `hashicorp` address. The runner therefore changes only the lookup key from the exact verified `registry.terraform.io/dokdo2013/iwinv` address to `iwinv` in a temporary file, preserving all schema content. It does not change the provider's installation address or pretend it belongs to HashiCorp. The tool ignores `docs/ko`, so an unchanged temporary copy of the Korean pages is separately validated as a `docs` root. Both 28-page trees must pass. No curated files are generated or overwritten.

This catches directory/file layout, frontmatter, file limits and registered-page coverage. It found missing frontmatter in both rule guides and four Korean resource pages; they are corrected. Two additional injected failures remove the required guide title separately in English and Korean, and CI must reject both. The six regression cases include the four earlier HCL/translation/schema checks. Neither this tool nor the temporary Korean layout establishes Registry locale support, rendered navigation/links or complete behavioral correctness.

The public [Registry documentation preview](https://registry.terraform.io/tools/doc-preview), linked from the [official FAQ](https://developer.hashicorp.com/terraform/registry/faq), was exercised on all 56 English/Korean pages on 2026-09-19. The rendered bodies showed every primary heading, hid frontmatter and contained all 106 expected tables; Korean text and the generated page outline were also inspected visually. [Per-page hashes and observations](../inventory/doc-preview.json) identify that specific content snapshot, not future revisions.

The preview exposed repository-only relative links resolving under the Registry host. All 91 links from English Registry pages to Korean docs, design, examples or other repository-only files now use explicit GitHub URLs, and the preview confirmed the new destinations. `check_docs.py` rejects those relative escapes and checks the local target of this repository's absolute GitHub `main` links. Links among English Registry pages remain relative. Development links currently track `main`; review version-specific destinations before release. This body preview does not exercise an actual published provider's sidebar, version routing or complete link navigation, and it does not establish Registry locale routes for Korean pages.

Sources: [official tool and validation scope](https://github.com/hashicorp/terraform-plugin-docs/tree/v0.25.0), [schema lookup implementation](https://github.com/hashicorp/terraform-plugin-docs/blob/v0.25.0/internal/provider/schema.go), [Registry documentation format](https://developer.hashicorp.com/terraform/registry/providers/docs).

Diagnostics have stable searchable codes and English technical details, with Korean/English troubleshooting pages.
Avoid locale-dependent identifiers or unstable translated error matching. A future CLI language option needs an ADR.
Unicode examples use synthetic names and documentation IP ranges; never real accounts or customer identifiers.

## Translation and release checks

- Require paired file presence and stable Cxx/Txxx IDs in CI; human review confirms semantic parity.
- Verify generated documentation is reproducible, all snippets validate with the built provider and local links resolve.
- Check Korean name normalization, character/byte limits and error display with actual fixtures.
- Publish both-language changelogs for breaking changes, state migrations, changed defaults and new limitations.
- Do not auto-translate secret payloads or publish real account screenshots.

Glossary: resource = 리소스; data source = 조회용 데이터 소스; state = Terraform 상태;
drift = 외부 변경에 따른 차이; import = 기존 리소스 편입; replacement = 삭제 후 재생성;
idempotency = 중복 실행해도 결과가 같음; eventual consistency = 반영 지연; ephemeral = 영구 저장하지 않는 임시 값.
