#!/usr/bin/env python3
"""Validate introductory/catalog HCL and attribute coverage using a built provider.

This does not verify every page's prose, Registry rendering or live API behavior.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
NAMES = ("availability_zones", "images", "image", "instance_types", "instance_type", "ssh_keys", "ssh_key")
SOURCE = "registry.terraform.io/dokdo2013/iwinv"
COMMON = '''terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}
provider "iwinv" {}
'''


def require(condition, message):
    if not condition:
        raise ValueError(message)


def check(terraform, provider_dir):
    with tempfile.TemporaryDirectory(prefix="iwinv-doc-hcl-") as tmp:
        base = Path(tmp)
        cli = base / "terraform.tfrc"
        cli.write_text('provider_installation {\n  dev_overrides {\n' +
                       f'    "dokdo2013/iwinv" = {json.dumps(str(provider_dir))}\n' +
                       '  }\n  direct {}\n}\n')
        env = {k: os.environ[k] for k in ("PATH", "TMPDIR", "LANG", "SystemRoot", "WINDIR") if k in os.environ}
        env.update(HOME=str(base), TF_CLI_CONFIG_FILE=str(cli), TF_IN_AUTOMATION="1", CHECKPOINT_DISABLE="1")
        documents = []
        for locale in ("", "ko/"):
            documents += [ROOT / f"docs/{locale}index.md"]
            documents += [ROOT / f"docs/{locale}data-sources/{name}.md" for name in NAMES]
        schema = None
        for i, document in enumerate(documents):
            text = document.read_text()
            label = str(document.relative_to(ROOT))
            blocks = re.findall(r"```hcl\n(.*?)\n```", text, re.S)
            require(len(blocks) == 1, f"Expected one complete HCL example: {label}")
            work = base / str(i)
            work.mkdir()
            prefix = "" if document.name == "index.md" else COMMON
            (work / "main.tf").write_text(prefix + blocks[0] + "\n")
            # validate and schema do not configure a client or execute data reads.
            # No init, plan, apply, account credentials or user's CLI config.
            result = subprocess.run([terraform, "validate", "-json"], cwd=work, env=env,
                                    capture_output=True, text=True, timeout=30)
            require(result.returncode == 0 and json.loads(result.stdout)["valid"], f"Invalid example: {label}")
            if schema is None:
                result = subprocess.run([terraform, "providers", "schema", "-json"], cwd=work, env=env,
                                        check=True, capture_output=True, text=True, timeout=30)
                schema = json.loads(result.stdout)["provider_schemas"][SOURCE]
            table = set(re.findall(r"^\| `([a-z_]+)` \|", text, re.M))
            block = (schema["provider"] if document.name == "index.md" else
                     schema["data_source_schemas"]["iwinv_" + document.stem])["block"]
            require(table == set(block["attributes"]), f"Attribute table differs from provider schema: {label}")
            if "ko" not in document.relative_to(ROOT / "docs").parts:
                other = ROOT / "docs/ko" / document.relative_to(ROOT / "docs")
                require(blocks == re.findall(r"```hcl\n(.*?)\n```", other.read_text(), re.S),
                        f"Translated HCL example differs: {label}")
    print("16 introductory/catalog HCL examples passed; top-level attribute coverage matches runtime schema; bilingual HCL is identical.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--terraform", type=Path, required=True, help="Actual Terraform executable, not a home-dependent wrapper")
    parser.add_argument("--provider-dir", type=Path, required=True, help="Directory containing the built provider binary")
    args = parser.parse_args()
    try:
        check(str(args.terraform.resolve()), args.provider_dir.resolve())
    except (OSError, KeyError, ValueError, subprocess.SubprocessError) as error:
        # Do not dump subprocess output or inherited environment values.
        detail = str(error) if type(error) is ValueError else type(error).__name__
        parser.exit(1, f"Introductory documentation validation failed: {detail}\n")


if __name__ == "__main__":
    main()
