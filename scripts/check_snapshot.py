#!/usr/bin/env python3
"""Verify unsigned 0.0.0-dev packages. This is not Registry/signature acceptance."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import tempfile
import zipfile

PREFIX = "terraform-provider-iwinv"
VERSION = "0.0.0-dev"
PLATFORMS = {"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64",
             "linux_arm", "linux_s390x", "windows_amd64"}
MANIFEST = f"{PREFIX}_{VERSION}_manifest.json"
MANIFEST_CONTENT = {"version": 1, "metadata": {"protocol_versions": ["6.0"]}}
SUMS = f"{PREFIX}_{VERSION}_SHA256SUMS"
SOURCE = "registry.terraform.io/dokdo2013/iwinv"


def archive_name(target):
    return f"{PREFIX}_{VERSION}_{target}.zip"


def binary_name(target):
    return f"{PREFIX}_v{VERSION}" + (".exe" if target.startswith("windows_") else "")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def validate_packages(directory):
    expected = {archive_name(p) for p in PLATFORMS} | {MANIFEST}
    checksums = {}
    for line in (directory / SUMS).read_text().splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9_.-]+)", line)
        require(match is not None, "Malformed checksum row")
        digest, name = match.groups()
        require(name in expected and name not in checksums, "Unexpected or duplicate checksum asset")
        checksums[name] = digest
    require(set(checksums) == expected, "Missing checksum assets")
    require({p.name for p in directory.glob("*.zip")} == expected - {MANIFEST}, "Unexpected archive set")
    for name, digest in checksums.items():
        path = directory / name
        require(not path.is_symlink() and path.is_file(), "Missing or linked package asset")
        with path.open("rb") as source:
            h = hashlib.sha256()
            for chunk in iter(lambda: source.read(1024 * 1024), b""):
                h.update(chunk)
        require(h.hexdigest() == digest, "Package checksum mismatch")
    require(json.loads((directory / MANIFEST).read_text()) == MANIFEST_CONTENT, "Protocol manifest mismatch")
    for target in sorted(PLATFORMS):
        with zipfile.ZipFile(directory / archive_name(target)) as archive:
            names = archive.namelist()
            require(len(names) == 2 and set(names) == {binary_name(target), "LICENSE"}, "Unexpected archive members")
            for member in archive.infolist():
                require(not member.is_dir() and 0 < member.file_size <= 256 * 1024 * 1024,
                        "Invalid archive member size")
                mode = member.external_attr >> 16
                require(mode & 0o170000 in (0, 0o100000), "Non-regular archive member")
            binary = archive.getinfo(binary_name(target))
            if not target.startswith("windows_"):
                require(binary.external_attr >> 16 & 0o111 != 0, "Provider binary is not executable")
            require(archive.testzip() is None, "Archive CRC mismatch")


def isolated_environment(root):
    env = {k: os.environ[k] for k in ("PATH", "SystemRoot", "WINDIR", "TMPDIR", "LANG") if k in os.environ}
    env.update(HOME=str(root), CHECKPOINT_DISABLE="1", TF_IN_AUTOMATION="1")
    return env


def verify_builds(directory, root):
    for target in sorted(PLATFORMS):
        binary = root / binary_name(target)
        with zipfile.ZipFile(directory / archive_name(target)) as archive:
            binary.write_bytes(archive.read(binary_name(target)))
        result = subprocess.run(["go", "version", "-m", str(binary)], check=True,
                                capture_output=True, text=True, env=isolated_environment(root), timeout=30).stdout
        goos, goarch = target.split("_")
        for setting in ("CGO_ENABLED=0", f"GOOS={goos}", f"GOARCH={goarch}", "-trimpath=true"):
            require(f"\tbuild\t{setting}\n" in result, "Go build metadata mismatch")
        if goarch == "arm":
            require("\tbuild\tGOARM=6" in result, "ARM baseline mismatch")
        binary.unlink()


def verify_install(directory, terraform, root):
    arch = {"x86_64": "amd64", "AMD64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine())
    target = f"{platform.system().lower()}_{arch}"
    require(target in PLATFORMS, "Host platform is not in the package matrix")
    binary = root / binary_name(target)
    with zipfile.ZipFile(directory / archive_name(target)) as archive:
        binary.write_bytes(archive.read(binary_name(target)))
    binary.chmod(0o700)
    version = subprocess.run([str(binary), "-version"], check=True, capture_output=True,
                             text=True, env=isolated_environment(root), timeout=30).stdout.strip()
    require(version == VERSION, "Native provider version was not injected")
    mirror = root / "mirror"
    package_dir = mirror / SOURCE
    package_dir.mkdir(parents=True)
    shutil.copyfile(directory / archive_name(target), package_dir / archive_name(target))
    cli = root / "terraform.tfrc"
    cli.write_text('provider_installation {\n  filesystem_mirror {\n' +
                   f'    path = {json.dumps(str(mirror))}\n    include = ["{SOURCE}"]\n' +
                   '  }\n}\n')
    work = root / "workspace"
    work.mkdir()
    (work / "main.tf").write_text('terraform {\n  required_providers {\n' +
        f'    iwinv = {{ source = "dokdo2013/iwinv", version = "={VERSION}" }}\n' +
        '  }\n}\n')
    env = isolated_environment(root)
    env.update(TF_CLI_CONFIG_FILE=str(cli), TF_DATA_DIR=str(work / ".terraform"))
    # No direct installer, dev override, user cache, credentials, or provider configuration.
    subprocess.run([terraform, "init", "-backend=false", "-input=false", "-no-color"], cwd=work,
                   env=env, check=True, capture_output=True, text=True, timeout=120)
    result = subprocess.run([terraform, "providers", "schema", "-json"], cwd=work, env=env,
                            check=True, capture_output=True, text=True, timeout=60)
    schema = json.loads(result.stdout)["provider_schemas"][SOURCE]
    ledger = json.loads((Path(__file__).resolve().parents[1] / "design/inventory/implementation.json").read_text())
    for kind, field in (("resource", "resource_schemas"), ("data_source", "data_source_schemas")):
        expected = {c["terraform_type"] for c in ledger["capabilities"] if c["kind"] == kind}
        require(set(schema.get(field, {})) == expected, "Installed schema does not match the capability ledger")
    require((work / ".terraform.lock.hcl").is_file(), "Terraform did not create an installation lock file")
    print(f"Filesystem mirror init and schema handshake passed on {target}; unsigned local package only.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--terraform", help="Also install and start the native package using this Terraform executable")
    args = parser.parse_args()
    stage = "archive/checksum validation"
    try:
        directory = args.directory.resolve()
        validate_packages(directory)
        with tempfile.TemporaryDirectory(prefix="iwinv-package-check-") as tmp:
            root = Path(tmp)
            stage = "Go build metadata validation"
            verify_builds(directory, root)
            if args.terraform:
                stage = "native version/filesystem mirror installation"
                verify_install(directory, str(Path(args.terraform).resolve()), root)
    except (OSError, ValueError, KeyError, zipfile.BadZipFile, subprocess.SubprocessError):
        parser.exit(1, f"Snapshot {stage} failed; no signature or Registry result was established.\n")
    print("Seven archives, protocol 6 manifest, SHA-256 checksums and Go build metadata verified.")


if __name__ == "__main__":
    main()
