# Release preparation

[한국어](../ko/release-readiness.md) · [Index](../../README.en.md)

There is no Registry release. T077 covers unsigned package preparation only. T053 remains incomplete until signing, publishing and clean Registry installation are verified. The current development provider exposes eighteen data sources and seven resources; the feature ledger defines their limited live-tested scope.

## Repeatable unsigned rehearsal

Use Go 1.25.8 or 1.26.1, GoReleaser **2.18.2**, Python 3.10+ and the actual Terraform **1.14.2 executable**, not a version-manager wrapper dependent on your home directory:

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=sign,publish --parallelism=2
cp terraform-registry-manifest.json dist/terraform-provider-iwinv_0.0.0-dev_manifest.json
python3 scripts/check_snapshot.py dist --terraform /absolute/path/to/terraform
python3 -m unittest discover -s scripts -p 'test_check_snapshot.py' -v
```

The snapshot version is always `0.0.0-dev`, independent of tags. GoReleaser computes the manifest checksum with its release asset name; the explicit copy stages that same file for offline checking when publication is skipped. `dist/` is ignored. `--clean` removes the previous build output, so preserve any needed local evidence elsewhere first.

The matrix is Darwin amd64/arm64, Linux amd64/arm64/arm (ARMv6)/s390x and Windows amd64. Builds disable CGO, trim local paths and inject the provider version. This is cross-compilation, not native execution coverage for every target. The checker validates the exact archive and member sets, executable mode, CRCs, checksums, protocol 6 manifest and Go target metadata. Synthetic tests reject corruption, missing/duplicate checksums, wrong protocol, extra/path-traversal members and lost executable mode.

On the host target, it checks `-version`, installs the ZIP through an isolated filesystem mirror using real `terraform init`, then obtains the provider schema and compares registered names to the implementation ledger. It uses no development override, direct installer, user cache or cloud credentials. An unsigned mirror install is **not** signature or Registry verification. The temporary configuration and lock file are removed automatically. No plan/apply or cloud request is made.

The package workflow has read-only repository permissions, pinned actions/tool versions, no signing secrets and no release upload. Its snapshot command explicitly skips signing and publishing. Existing protocol CI continues to test Terraform 1.14.0/1.14.2 independently.

## Remaining release gates

- Resolve or explicitly scope outstanding capability contracts and the cleanup failure T056, including the work-created webmail service and unconsented MCP client registration. Never describe them as deleted without evidence.
- Complete Registry-format overview and missing catalog pages in both languages, review schema/documentation parity (T052), and verify Registry rendering. Design pages alone are insufficient.
- Establish project-specific signing-key custody and recovery. Register its public key in the `dokdo2013` Registry namespace; verify current accepted key algorithms before creation. No key has been generated or uploaded by this rehearsal.
- Add a restricted signing/publication workflow only after the release gates and credential storage are ready. The GoReleaser configuration describes checksum signing using `GPG_FINGERPRINT` and a draft release, but those steps have **not** been executed or verified.
- Select an immutable semantic version, verify its release manifest/archives/checksums and detached binary GPG signature, then publish and verify a fresh Registry install. Never replace an existing published version. Check minimum Terraform requirements as well as plugin protocol metadata; protocol 6 alone does not encode the minimum CLI version.
- Test supported upgrade paths when a prior version exists (T054); there is currently no released state version to claim migration acceptance against. Complete workflow privilege/fork review (T055).

T077 evidence: `.goreleaser.yml`, `terraform-registry-manifest.json`, `scripts/check_snapshot.py`, `scripts/test_check_snapshot.py`, `.github/workflows/package.yml`, and the [execution record](contract-progress.md).

Sources: [HashiCorp publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing), [recommended targets](https://developer.hashicorp.com/terraform/registry/providers/os-arch), [official scaffold configuration](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/.goreleaser.yml), [GoReleaser snapshots](https://goreleaser.com/customization/publish/snapshots/).
