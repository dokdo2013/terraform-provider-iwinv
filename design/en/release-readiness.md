# Release preparation

[한국어](../ko/release-readiness.md) · [Index](../../README.en.md)

There is no Registry release. T077 covers unsigned package preparation; T078 adds disposable-key signing verification. T053 remains incomplete until production signing, publishing and clean Registry installation are verified. The current development provider exposes eighteen data sources and seven resources; the feature ledger defines their limited live-tested scope.

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

## Disposable-key signing rehearsal

With GnuPG 2.x (`gpg` and `gpgconf`) installed, run:

```sh
python3 scripts/check_signing.py --goreleaser /absolute/path/to/goreleaser --terraform /absolute/path/to/terraform
```

This rebuilds `dist/` using the same GoReleaser configuration, with `--snapshot --clean --skip=publish`. It generates a one-day RSA-3072 signing key under a temporary 0700 home. This test-only key has no passphrase and must never become a production identity. The configured signer emits a binary detached SHA-256 signature; `--no-options` prevents user GPG defaults, including armor, from changing that format.

A separate keyring imports only the public key. Verification requires a successful GPG result and the exact generated fingerprint before checking all seven ZIPs and the manifest against the signed checksums. Tests reject changed checksums, changed signature bytes, a changed ZIP, a missing signature, armor, an unexpected fingerprint and an unknown public key. The original output is verified before native filesystem-mirror installation. Terraform's mirror installer does not authenticate this GPG signature itself; Registry key trust remains untested.

The runner excludes inherited cloud/publishing credentials and user Git/GPG configuration, reusing only public Go caches. On ordinary completion or exceptions it stops agents scoped to its own keyrings, removes the temporary keyrings and removes the disposable signature from `dist/`. It uploads no keys or artifacts. Abrupt process termination can still require local temporary-file cleanup.

The package workflow runs this rehearsal with read-only repository permissions, pinned actions/tool versions and no stored signing secrets or release upload. Existing protocol CI continues to test Terraform 1.14.0/1.14.2 independently. Local signing was verified with GnuPG 2.5.22 on Darwin arm64; CI supplies its runner's GnuPG version in the log.

## Remaining release gates

- Resolve or explicitly scope outstanding capability contracts and the cleanup failure T056, including the work-created webmail service and unconsented MCP client registration. Never describe them as deleted without evidence.
- Overview and capability pages now exist in both languages. Finish the full behavior/schema documentation review (T052) and verify Registry rendering; page existence alone does not prove those gates.
- Establish project-specific signing-key custody and recovery. Register its public key in the `dokdo2013` Registry namespace; verify current accepted key algorithms before creation. No production key has been generated or uploaded; the test key is deleted.
- Add a restricted signing/publication workflow only after the release gates and credential storage are ready. Checksum signing using `GPG_FINGERPRINT` has been exercised with a disposable key. Production key loading and draft release upload have **not** been executed or verified.
- Select an immutable semantic version, verify its release manifest/archives/checksums and detached binary GPG signature, then publish and verify a fresh Registry install. Never replace an existing published version. Check minimum Terraform requirements as well as plugin protocol metadata; protocol 6 alone does not encode the minimum CLI version.
- Test supported upgrade paths when a prior version exists (T054); there is currently no released state version to claim migration acceptance against. Complete workflow privilege/fork review (T055).

T077 evidence: `.goreleaser.yml`, `terraform-registry-manifest.json`, `scripts/check_snapshot.py`, `scripts/test_check_snapshot.py`, `.github/workflows/package.yml`, and the [execution record](contract-progress.md).

T078 evidence: `scripts/check_signing.py`, `.goreleaser.yml`, `.github/workflows/package.yml`, and the same execution record. See [GoReleaser signing](https://goreleaser.com/customization/sign/sign/) and [GnuPG test key generation](https://www.gnupg.org/documentation/manuals/gnupg/Unattended-GPG-key-generation.html).

Sources: [HashiCorp publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing), [recommended targets](https://developer.hashicorp.com/terraform/registry/providers/os-arch), [official scaffold configuration](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/.goreleaser.yml), [GoReleaser snapshots](https://goreleaser.com/customization/publish/snapshots/).
