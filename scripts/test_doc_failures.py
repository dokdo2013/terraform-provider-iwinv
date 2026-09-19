#!/usr/bin/env python3
"""Inject documentation regressions in a temporary copy; require real CLI rejection."""
import argparse
from pathlib import Path
import shutil
import tempfile

import check_intro_docs as checker


def check_failures(terraform, provider_dir, tfplugindocs=None):
    source = checker.ROOT
    with tempfile.TemporaryDirectory(prefix="iwinv-doc-regression-") as tmp:
        root = Path(tmp)
        shutil.copytree(source / "docs", root / "docs")
        (root / "design/inventory").mkdir(parents=True)
        shutil.copyfile(source / "design/inventory/implementation.json", root / "design/inventory/implementation.json")
        cases = [
            ("missing variable", "docs/resources/content_cache.md",
             'variable "product_id" { type = string }\n', "", "Invalid example:"),
            ("missing parent", "docs/resources/security_group_egress_rule.md",
             'resource "iwinv_security_group" "example" {\n  name = "tf-example-group"\n}\n\n', "", "Invalid example:"),
            ("translated configuration drift", "docs/ko/resources/content_cache.md",
             '"tf-example-cache"', '"tf-different-cache"', "Translated HCL example differs:"),
            ("lost nested sensitivity", "docs/guides/schema_reference.md",
             '| `bills[].price` | `number` | computed, sensitive (inherited) |',
             '| `bills[].price` | `number` | computed |', "Runtime schema reference differs:"),
        ]
        if tfplugindocs:
            cases += [
                ("missing English guide title", "docs/guides/security_group_rules.md",
                 'page_title: "Security group rule lifecycle and recovery"\n', "",
                 "Registry format validation failed (en):"),
                ("missing Korean guide title", "docs/ko/guides/security_group_rules.md",
                 'page_title: "보안 그룹 규칙의 수명주기와 복구"\n', "",
                 "Registry format validation failed (ko):"),
            ]
        checker.ROOT = root
        try:
            for label, relative, old, new, reason in cases:
                path = root / relative
                original = path.read_text()
                if original.count(old) != 1:
                    raise AssertionError(f"Regression fixture needs review: {label}")
                try:
                    path.write_text(original.replace(old, new))
                    try:
                        checker.check(terraform, provider_dir, tfplugindocs=tfplugindocs)
                    except ValueError as error:
                        if not str(error).startswith(reason):
                            raise AssertionError(f"Unexpected rejection path: {label}") from error
                    else:
                        raise AssertionError(f"Invalid documentation accepted: {label}")
                finally:
                    path.write_text(original)
                print(f"Rejected documentation regression: {label}", flush=True)
        finally:
            checker.ROOT = source


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--terraform", type=Path, required=True)
    parser.add_argument("--provider-dir", type=Path, required=True)
    parser.add_argument("--tfplugindocs", type=Path)
    args = parser.parse_args()
    check_failures(str(args.terraform.resolve()), args.provider_dir.resolve(),
                   str(args.tfplugindocs.resolve()) if args.tfplugindocs else None)
