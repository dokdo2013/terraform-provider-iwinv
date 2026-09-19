#!/usr/bin/env python3
"""Explicit live read-only test of native Terraform provider aliases.

One real environment-backed alias must succeed and one synthetic invalid alias
must fail independently. No credentials or remote values are printed or saved.
This does not prove isolation between two valid customer accounts.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def main():
    if os.environ.get("IWINV_LIVE_READ") != "1":
        raise SystemExit("Set IWINV_LIVE_READ=1 to opt into authenticated read-only requests")
    if not all(os.environ.get(name) for name in ("IWINV_ACCESS_KEY", "IWINV_SECRET_KEY")):
        raise SystemExit("Environment credentials are required")
    binary = Path(__file__).resolve().parents[1] / "bin/terraform-provider-iwinv"
    if not binary.is_file():
        raise SystemExit("Build bin/terraform-provider-iwinv first")
    terraform = os.environ.get("TF_ACC_TERRAFORM_PATH") or shutil.which("terraform")
    if not terraform:
        raise SystemExit("Terraform executable not found")
    with tempfile.TemporaryDirectory(prefix="iwinv-alias-test-") as directory:
        root = Path(directory)
        config = root / "terraform.tfrc"
        config.write_text(
            'provider_installation {\n dev_overrides {\n "dokdo2013/iwinv" = '
            + json.dumps(str(binary.parent)) + '\n }\n direct {}\n}\n'
        )
        (root / "main.tf").write_text('''terraform {
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" }
  }
}
provider "iwinv" {}
provider "iwinv" {
  alias      = "invalid"
  access_key = "synthetic-invalid-access"
  secret_key = "synthetic-invalid-secret"
}
data "iwinv_availability_zones" "primary" {}
data "iwinv_availability_zones" "invalid" {
  provider   = iwinv.invalid
  depends_on = [data.iwinv_availability_zones.primary]
}
''')
        env = os.environ.copy()
        env["TF_CLI_CONFIG_FILE"] = str(config)
        env.pop("TF_LOG", None)
        env.pop("TF_LOG_PATH", None)
        try:
            result = subprocess.run(
                [terraform, f"-chdir={directory}", "plan", "-json", "-input=false"],
                env=env, capture_output=True, text=True, timeout=60, check=False,
            )
        except (OSError, subprocess.TimeoutExpired):
            raise SystemExit("Terraform invocation failed; raw diagnostics suppressed") from None
        events = []
        for line in result.stdout.splitlines():
            try:
                events.append(json.loads(line))
            except ValueError:
                pass
        primary_read = any(
            event.get("type") == "apply_complete"
            and event.get("hook", {}).get("resource", {}).get("addr")
            == "data.iwinv_availability_zones.primary"
            for event in events
        )
        secondary_rejected = any(
            event.get("diagnostic", {}).get("address") == "data.iwinv_availability_zones.invalid"
            and "CHECK_CREDENTIAL" in event.get("diagnostic", {}).get("detail", "")
            for event in events
        )
        passed = result.returncode == 1 and primary_read and secondary_rejected
        print(json.dumps({"test": "native_alias_isolation", "passed": passed,
                          "primary_read_completed": primary_read,
                          "invalid_alias_rejected": secondary_rejected,
                          "cloud_mutations": 0}))
        if not passed:
            raise SystemExit(1)


if __name__ == "__main__":
    main()
