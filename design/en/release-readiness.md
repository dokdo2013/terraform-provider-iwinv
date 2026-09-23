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

The package workflow runs this rehearsal with read-only repository permissions, pinned actions/tool versions and no stored signing secrets or release upload. Existing protocol CI continues to test Terraform 1.14.0/1.14.2/1.16.3 independently. Local signing was verified with GnuPG 2.5.22 on Darwin arm64; CI supplies its runner's GnuPG version in the log.

## Workflow permissions and review (T055)

The repository settings were read through GitHub's API on 2026-09-19: default workflow token permissions are read-only, workflow approval of pull requests is disabled, and first-time fork contributors require approval. There were no repository Actions secrets or environments. Full commit-SHA pinning was enabled at the repository level and read back successfully; allowed actions remain `all`. SHA pinning fixes an action revision, not its trustworthiness. These observations are time-bound, not permanent guarantees.

All four current workflows use `push` to `main` and `pull_request`, `contents: read`, GitHub-hosted Ubuntu runners and checkout without persisted credentials. None references stored secrets, OIDC write permissions, a release environment, `pull_request_target`, `workflow_run`, or a publication step. PR code still executes arbitrary code with network access on its runner; a static audit is not a sandbox. The ordinary fork event withholds repository secrets and limits the token according to GitHub's policy. No real external-fork run was performed in this audit.

`Workflow audit` runs actionlint **1.7.12** for syntax/expressions/shell analysis and zizmor **1.30.1** in offline pedantic mode, failing on low-or-higher severity findings, for workflow security patterns. The zizmor wheel is installed in a temporary virtual environment using `scripts/requirements-workflow.txt`, exact SHA-256 hashes, binary-only installation and no dependencies. The audit receives no API token and performs no online advisory or action-owner review. Go's module checksum verification remains enabled for actionlint. The tools themselves are dependencies that require review when upgraded.

The first scan flagged the package job's shared Go cache as a potential artifact cache-poisoning path. Its GoReleaser step is install-only and the signing harness skips publication, so this was not evidence of an exploitable published release. Shared cache restore/save was nevertheless removed from the package job to keep signed rehearsal artifacts independent of another run's build cache. Within-run Go caching remains; protocol test caching remains separate from publication. After this change actionlint and zizmor's offline pedantic audit passed at the low severity threshold. Five informational missing job display names remain below that threshold; these do not change job permissions. Synthetic temporary files were rejected for title-template injection and invalid expression contexts without executing them. A write-all permission finding appeared only in pedantic mode, which is why CI explicitly selects that mode.

To reproduce with installed tools:

```sh
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -color
zizmor --offline --no-progress --persona pedantic --min-severity low .github/workflows
```

Use zizmor 1.30.1 installed from the hash-locked requirements; the workflow shows the exact isolated installation command. Tool success does not prove runtime secret isolation, dependency safety or release authorization. T055 remains in progress until an actual external-fork run and the eventual production signing/publication workflow are reviewed. Before adding production keys, review protected environments, immutable release/tag selection, minimal job-level publication permissions, separation from PR artifacts/caches, key cleanup and failed-release recovery. Do not grant those permissions to the current PR jobs.

Sources: [GitHub repository Actions settings](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository), [GitHub permissions API](https://docs.github.com/en/rest/actions/permissions), [zizmor operating modes and limits](https://docs.zizmor.sh/usage/), [actionlint](https://github.com/rhysd/actionlint).

## Terraform CLI compatibility

The minimum remains Terraform 1.14.0. The synthetic protocol/documentation CI matrix additionally covers 1.14.2 and **1.16.3**, the stable release checked on 2026-09-19. The [official 1.16.3 release](https://github.com/hashicorp/terraform/releases/tag/v1.16.3) includes import-provider-resolution and lifecycle fixes, making it relevant to this provider's import and replacement behavior. A minimum version constraint is not a claim that every newer CLI has been tested.

The matrix executes the existing synthetic `TestProtocol` lifecycle/import/drift/failure tests, validates all examples against the actual built provider, compares both complete schema references, runs official documentation validation and rejects the six injected documentation failures. These jobs have no live credentials. Local 1.16.3 validation uses the official Darwin ARM64 archive after matching its SHA-256 against the vendor's HTTPS checksum file; the temporary binary does not replace the user's configured Terraform.

Local 1.16.3 protocol tests passed with the race detector (84.978 seconds), as did all 52 HCL snippets, both runtime schema references, official format checks and all six documentation rejection cases. The verified Darwin ARM64 archive SHA-256 was `c2c45425ea4568da9803e127e589186cb3798a5944d9aff5a5bc15dd18267560`. This is an HTTPS checksum comparison, not a claim of an independently verified vendor GPG signature.

This tests supported behavior on separate CLI versions. It does not establish an upgrade of an existing state from a previous published provider, every intermediate CLI release, native execution on all seven package targets or a live API lifecycle. T054 remains `not_run`: no previous provider release exists for its migration gate.

## Remaining release gates

- Resolve or explicitly scope outstanding capability contracts and the cleanup failure T056. The work-created webmail service was later confirmed deleted by console history and an empty list. Cleanup of the unconsented MCP client registration is still unverified; one cleaned webmail fixture does not pass overall T056.
- Overview and capability pages now exist in both languages. Official format checks and all 56 body previews passed; the [scoped narrative review](guide-review.md) covers the current 28 page pairs. Published Registry navigation/version-link checks (T052) remain open. See the documentation policy for the exact preview snapshot and limits.
- Establish project-specific signing-key custody and recovery. Register its public key in the `dokdo2013` Registry namespace; verify current accepted key algorithms before creation. No production key has been generated or uploaded; the test key is deleted.
- Add a restricted signing/publication workflow only after the release gates and credential storage are ready. Checksum signing using `GPG_FINGERPRINT` has been exercised with a disposable key. Production key loading and draft release upload have **not** been executed or verified.
- Select an immutable semantic version, verify its release manifest/archives/checksums and detached binary GPG signature, then publish and verify a fresh Registry install. Never replace an existing published version. Check minimum Terraform requirements as well as plugin protocol metadata; protocol 6 alone does not encode the minimum CLI version.
- Test supported upgrade paths when a prior version exists (T054); there is currently no released state version to claim migration acceptance against. Complete workflow privilege/fork review (T055).

T077 evidence: `.goreleaser.yml`, `terraform-registry-manifest.json`, `scripts/check_snapshot.py`, `scripts/test_check_snapshot.py`, `.github/workflows/package.yml`, and the [execution record](contract-progress.md).

T078 evidence: `scripts/check_signing.py`, `.goreleaser.yml`, `.github/workflows/package.yml`, and the same execution record. See [GoReleaser signing](https://goreleaser.com/customization/sign/sign/) and [GnuPG test key generation](https://www.gnupg.org/documentation/manuals/gnupg/Unattended-GPG-key-generation.html).

Sources: [HashiCorp publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing), [recommended targets](https://developer.hashicorp.com/terraform/registry/providers/os-arch), [official scaffold configuration](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/.goreleaser.yml), [GoReleaser snapshots](https://goreleaser.com/customization/publish/snapshots/).
