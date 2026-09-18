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

Once schemas exist, generate standard English Registry docs under `docs/` using `terraform-plugin-docs`.
Keep Korean equivalents in a separate localized tree (proposed `guides/ko/reference/`), linked from Registry docs;
do not assume the Registry supports a locale router. Build a bilingual documentation site later without duplicating schemas.
Executable examples have one shared source; bilingual prose points to the same tested example files.
Until a provider exists, examples remain fenced design snippets explicitly marked non-runnable.

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
