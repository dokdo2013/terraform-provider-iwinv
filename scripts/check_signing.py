#!/usr/bin/env python3
"""Rehearse snapshot signing with a disposable key, never a release identity."""
import argparse
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

from check_snapshot import (MANIFEST, SUMS, archive_name, isolated_environment,
                            require, validate_packages, verify_builds, verify_install)

ROOT = Path(__file__).resolve().parents[1]
SIGNATURE = SUMS + ".sig"


def run(args, env, *, data=None, timeout=60, check=True):
    return subprocess.run(args, input=data, capture_output=True, env=env,
                          cwd=ROOT, timeout=timeout, check=check)


def gpg(home, env, *args, data=None, check=True):
    return run(["gpg", "--no-options", "--homedir", str(home), "--batch", "--no-tty",
                "--no-auto-key-retrieve", *args], env, data=data, check=check)


def verify_signed_packages(directory, home, env, fingerprint):
    sig = directory / SIGNATURE
    sums = directory / SUMS
    for path in (sig, sums):
        require(path.is_file() and not path.is_symlink(), "Missing or linked signing input")
        require(0 < path.stat().st_size < 16384, "Invalid signing input size")
    require(sig.read_bytes()[0] & 0x80, "Signature must be binary OpenPGP")
    result = gpg(home, env, "--status-fd", "1", "--verify", str(sig), str(sums), check=False)
    valid = [line.split()[2] for line in result.stdout.decode().splitlines()
             if line.startswith("[GNUPG:] VALIDSIG ")]
    require(result.returncode == 0 and valid == [fingerprint], "Invalid or unexpected signing key")
    # Only trust archive checksums after validating their signature and signer.
    validate_packages(directory)


def rejection_cases(directory, root, home, env, fingerprint):
    copy = root / "mutations"
    copy.mkdir()
    for path in directory.iterdir():
        if path.suffix == ".zip" or path.name in (SUMS, SIGNATURE, MANIFEST):
            shutil.copyfile(path, copy / path.name)
    for name, target, change in (
        ("checksum mutation", SUMS, lambda b: b + b"\n"),
        ("signature mutation", SIGNATURE, lambda b: b[:-1] + bytes([b[-1] ^ 1])),
        ("archive mutation", archive_name("linux_amd64"), lambda b: b + b"tampered"),
        ("missing signature", SIGNATURE, lambda b: None),
        ("armored signature", SIGNATURE, lambda b: b"-----BEGIN PGP SIGNATURE-----\n"),
    ):
        path = copy / target
        original = path.read_bytes()
        changed = change(original)
        try:
            if changed is None:
                path.unlink()
            else:
                path.write_bytes(changed)
            expect_rejection(lambda: verify_signed_packages(copy, home, env, fingerprint), name)
        finally:
            path.write_bytes(original)
    wrong = ("0" if fingerprint[0] != "0" else "1") + fingerprint[1:]
    expect_rejection(lambda: verify_signed_packages(copy, home, env, wrong), "unexpected fingerprint")
    empty = root / "untrusted"
    empty.mkdir(mode=0o700)
    expect_rejection(lambda: verify_signed_packages(copy, empty, env, fingerprint), "unknown public key")
    verify_signed_packages(copy, home, env, fingerprint)


def expect_rejection(check, name):
    try:
        check()
    except ValueError:
        print(f"Rejected: {name}", flush=True)
        return
    raise ValueError(f"Accepted invalid case: {name}")


def rehearsal(goreleaser, terraform):
    directory = ROOT / "dist"
    # A short POSIX temporary path also stays within GnuPG's Unix socket limit.
    with tempfile.TemporaryDirectory(prefix="iwinv-sign-", dir="/tmp") as tmp:
        root = Path(tmp)
        signing = root / "signer"
        verifier = root / "verifier"
        signing.mkdir(mode=0o700)
        verifier.mkdir(mode=0o700)
        env = isolated_environment(root)
        # Reuse only public Go build/module caches, not user credentials/config.
        for name in ("GOCACHE", "GOMODCACHE"):
            env[name] = subprocess.check_output(["go", "env", name], text=True).strip()
        env.update(GNUPGHOME=str(signing), GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)
        try:
            gpg(signing, env, "--pinentry-mode", "loopback", "--passphrase", "",
                "--quick-generate-key", "iwinv disposable rehearsal <test@example.invalid>",
                "rsa3072", "sign", "1d")
            listing = gpg(signing, env, "--with-colons", "--list-secret-keys").stdout.decode()
            fingerprints = [line.split(":")[9] for line in listing.splitlines() if line.startswith("fpr:")]
            require(len(fingerprints) == 1 and re.fullmatch(r"[A-F0-9]{40}", fingerprints[0]),
                    "Unexpected disposable key layout")
            fingerprint = fingerprints[0]
            public = gpg(signing, env, "--export", fingerprint).stdout
            gpg(verifier, env, "--import", data=public)
            require(not gpg(verifier, env, "--with-colons", "--list-secret-keys").stdout.strip(),
                    "Verifier unexpectedly has private keys")
            env["GPG_FINGERPRINT"] = fingerprint
            print("Building seven snapshot targets and signing with a disposable RSA key...", flush=True)
            run([goreleaser, "release", "--snapshot", "--clean", "--skip=publish", "--parallelism=2"],
                env, timeout=900)
            shutil.copyfile(ROOT / "terraform-registry-manifest.json", directory / MANIFEST)
            verify_signed_packages(directory, verifier, env, fingerprint)
            print("Binary detached signature, expected signer and eight asset checksums verified.", flush=True)
            rejection_cases(directory, root, verifier, env, fingerprint)
            verify_builds(directory, root)
            verify_install(directory, terraform, root)
        finally:
            # Scope agent shutdown to this test's keyring. Never stop user agents.
            try:
                for home in (signing, verifier, root / "untrusted"):
                    if home.exists():
                        run(["gpgconf", "--homedir", str(home), "--kill", "all"], env)
            finally:
                (directory / SIGNATURE).unlink(missing_ok=True)
    require(not root.exists() and not (directory / SIGNATURE).exists(), "Test key cleanup failed")
    print("Disposable keyrings/signature removed. No release identity or Registry result established.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goreleaser", required=True, help="GoReleaser 2.18.2 executable")
    parser.add_argument("--terraform", required=True, help="Actual Terraform 1.14.2 executable")
    args = parser.parse_args()
    try:
        rehearsal(str(Path(args.goreleaser).resolve()), str(Path(args.terraform).resolve()))
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        # Child output is intentionally not replayed; signing material stays local.
        parser.exit(1, f"Signing rehearsal failed ({type(error).__name__}); no release was published.\n")


if __name__ == "__main__":
    main()
