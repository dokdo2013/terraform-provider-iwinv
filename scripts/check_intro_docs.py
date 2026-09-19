#!/usr/bin/env python3
"""Validate all provider-page HCL and bilingual schema references with a binary.

The historical filename remains usable. This does not verify all narrative
semantics, Registry rendering, validators, plan modifiers or live API behavior.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess
import tempfile

from check_snapshot import isolated_environment

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


def type_name(value):
    if isinstance(value, str):
        require(value in {"string", "number", "bool", "dynamic"}, "Unknown schema primitive")
        return value
    require(isinstance(value, list) and len(value) == 2, "Invalid schema type")
    kind, element = value
    if kind in {"list", "set", "map"}:
        return f"{kind}({type_name(element)})"
    if kind == "object":
        return "object({" + ", ".join(f"{k}={type_name(v)}" for k, v in sorted(element.items())) + "})"
    if kind == "tuple":
        return "tuple([" + ", ".join(type_name(v) for v in element) + "])"
    raise ValueError("Unknown schema collection")


def schema_rows(block, locale, prefix="", inherited_sensitive=False):
    labels = ({"required": "필수", "optional": "선택", "computed": "계산", "sensitive": "민감",
               "write_only": "쓰기 전용", "deprecated": "지원 중단 예정"} if locale == "ko" else
              {"required": "required", "optional": "optional", "computed": "computed", "sensitive": "sensitive",
               "write_only": "write-only", "deprecated": "deprecated"})
    rows = []
    for name, attr in sorted(block.get("attributes", {}).items()):
        path = prefix + name
        flags = [labels[k] for k in labels if attr.get(k)]
        if inherited_sensitive and not attr.get("sensitive"):
            flags.append("민감(부모에서 상속)" if locale == "ko" else "sensitive (inherited)")
        nested = attr.get("nested_type")
        if nested:
            mode = nested["nesting_mode"]
            require(mode in {"single", "list", "set", "map"}, "Unknown nested attribute mode")
            shape = "object" if mode == "single" else f"{mode}(object)"
        else:
            shape = type_name(attr["type"])
        rows.append((path, shape, ", ".join(flags) or ("없음" if locale == "ko" else "none")))
        if nested:
            suffix = {"single": ".", "list": "[].", "set": "[].", "map": "[key]."}[mode]
            rows.extend(schema_rows(nested, locale, path + suffix,
                                    inherited_sensitive or attr.get("sensitive", False)))
    for name, nested in sorted(block.get("block_types", {}).items()):
        mode = nested["nesting_mode"]
        require(mode in {"single", "list", "set", "map"}, "Unknown block nesting mode")
        path = prefix + name
        minimum, maximum = nested.get("min_items", 0), nested.get("max_items", 0)
        shape = f"block({mode})"
        if minimum or maximum:
            shape += f"; min={minimum}, max={maximum or 'unbounded'}"
        flags = labels["required" if minimum else "optional"]
        if nested.get("deprecated"):
            flags += ", " + labels["deprecated"]
        rows.append((path, shape, flags))
        suffix = {"single": ".", "list": "[].", "set": "[].", "map": "[key]."}[mode]
        rows.extend(schema_rows(nested["block"], locale, path + suffix, inherited_sensitive))
    return rows


def schema_reference(schema, locale):
    ko = locale == "ko"
    title = "Provider 스키마 참조" if ko else "Provider schema reference"
    description = "실행 바이너리에서 확인한 전체 속성·중첩 구조와 입력·비밀값 표시입니다." if ko else "Complete runtime attribute, nesting, input and secret flags."
    lines = ["---", 'page_title: "Schema reference - iwinv"', 'subcategory: ""', "description: |-", f"  {description}", "---", "", f"# {title}", "",
             "[English](../../guides/schema_reference.md) · [시작 가이드](../index.md)" if ko else "[한국어](../ko/guides/schema_reference.md) · [Getting started](../index.md)", "",
             "이 표는 빌드한 Provider의 `terraform providers schema -json` 결과로 생성합니다. 아직 Registry 릴리스는 없습니다." if ko else
             "Generated from the built provider's `terraform providers schema -json` output. There is no Registry release yet.", "",
             "`[]`는 목록·집합 원소, `[key]`는 맵 값입니다. 필수/선택/계산은 프로토콜 스키마 표시이며 생성 시 조건부 필수 여부는 개별 가이드를 따릅니다. 민감 표시는 화면 가림이며 state 암호화가 아닙니다. 쓰기 전용 값은 리소스 plan/state에서 제외하지만 설정에 직접 적은 비밀값의 보관 위험까지 없애지는 않습니다. 부모가 민감하면 자식에도 상속 표시합니다." if ko else
             "`[]` denotes list/set elements; `[key]` denotes map values. Required/optional/computed are protocol schema flags; consult the individual guides for conditional creation requirements. Sensitive means display redaction, not state encryption. Write-only values are excluded from resource plan/state but cannot protect secrets embedded in configuration. Parent sensitivity is shown on child fields.", "",
             "`number`는 Terraform 타입입니다. 구현의 정수 범위·단위·null 처리·기본값·timeout 기본 기간·교체 조건·검증기·plan modifier는 이 출력에 모두 표현되지 않으므로 각 기능 가이드를 함께 읽어야 합니다. 스키마 버전은 Provider 릴리스 버전과 다릅니다." if ko else
             "`number` is the Terraform type. Integer limits, units, null behavior, defaults, timeout durations, replacement rules, validators and plan modifiers are not all encoded in this output; read each capability guide. Schema versions are not provider release versions.", "",
             "<!-- Generated by scripts/check_intro_docs.py --write-schema-reference. -->", ""]
    entries = [("Provider", schema["provider"], "../index.md")]
    for section, key in (("resources", "resource_schemas"), ("data-sources", "data_source_schemas")):
        entries.extend((name + (" (Resource)" if section == "resources" else " (Data Source)"), value,
                        f"../{section}/{name.removeprefix('iwinv_')}.md") for name, value in sorted(schema[key].items()))
    for name, value, link in entries:
        label = "동작·예제" if ko else "Behavior and example"
        lines += [f"## {name}", "", f"[{label}]({link}) · Schema version: `{value['version']}`", "",
                  "| 속성 경로 | 타입 | 표시 |" if ko else "| Attribute path | Type | Flags |", "| --- | --- | --- |"]
        lines += [f"| `{path}` | `{shape}` | {flags} |" for path, shape, flags in schema_rows(value["block"], locale)]
        lines += [""]
    return "\n".join(lines)


def verify_reference(actual, expected, label):
    require(actual == expected, f"Runtime schema reference differs: {label}; regenerate and review both languages")


def check(terraform, provider_dir, write_reference=False):
    with tempfile.TemporaryDirectory(prefix="iwinv-doc-hcl-") as tmp:
        base = Path(tmp)
        cli = base / "terraform.tfrc"
        cli.write_text('provider_installation {\n  dev_overrides {\n' +
                       f'    "dokdo2013/iwinv" = {json.dumps(str(provider_dir))}\n' +
                       '  }\n  direct {}\n}\n')
        env = isolated_environment(base)
        env["TF_CLI_CONFIG_FILE"] = str(cli)
        seed = base / "schema"
        seed.mkdir()
        (seed / "main.tf").write_text(COMMON)
        result = subprocess.run([terraform, "providers", "schema", "-json"], cwd=seed, env=env,
                                check=True, capture_output=True, text=True, timeout=30)
        envelope = json.loads(result.stdout)
        require(envelope["format_version"].split(".")[0] == "1", "Unsupported Terraform schema format")
        schema = envelope["provider_schemas"][SOURCE]
        require(not any(value for key, value in schema.items()
                        if key not in {"provider", "resource_schemas", "data_source_schemas"}),
                "New provider schema category needs documentation support")
        ledger = json.loads((ROOT / "design/inventory/implementation.json").read_text())["capabilities"]
        for kind, key in (("resource", "resource_schemas"), ("data_source", "data_source_schemas")):
            require(set(schema[key]) == {c["terraform_type"] for c in ledger if c["kind"] == kind},
                    "Runtime schema and implementation ledger differ")
        documents = []
        for locale in ("", "ko/"):
            documents += [ROOT / f"docs/{locale}index.md"]
            for section, key in (("resources", "resource_schemas"), ("data-sources", "data_source_schemas")):
                documents += [ROOT / f"docs/{locale}{section}/{name.removeprefix('iwinv_')}.md" for name in sorted(schema[key])]
        for i, document in enumerate(documents):
            text = document.read_text()
            label = str(document.relative_to(ROOT))
            blocks = re.findall(r"```hcl\n(.*?)\n```", text, re.S)
            require(len(blocks) == 1, f"Expected one complete HCL example: {label}")
            formatted = subprocess.run([terraform, "fmt", "-"], input=blocks[0] + "\n", env=env,
                                       capture_output=True, text=True, check=True, timeout=30).stdout
            require(formatted == blocks[0] + "\n", f"Unformatted HCL example: {label}")
            work = base / str(i)
            work.mkdir()
            prefix = "" if document.name == "index.md" else COMMON
            (work / "main.tf").write_text(prefix + blocks[0] + "\n")
            # validate and schema do not configure a client or execute data reads.
            # No init, plan, apply, account credentials or user's CLI config.
            result = subprocess.run([terraform, "validate", "-json"], cwd=work, env=env,
                                    capture_output=True, text=True, timeout=30)
            require(result.returncode == 0 and json.loads(result.stdout)["valid"], f"Invalid example: {label}")
            if document.name == "index.md" or document.parent.name == "data-sources" and document.stem in NAMES:
                table = set(re.findall(r"^\| `([a-z_]+)` \|", text, re.M))
                block = (schema["provider"] if document.name == "index.md" else
                         schema["data_source_schemas"]["iwinv_" + document.stem])["block"]
                require(table == set(block["attributes"]), f"Attribute table differs from provider schema: {label}")
            if "ko" not in document.relative_to(ROOT / "docs").parts:
                other = ROOT / "docs/ko" / document.relative_to(ROOT / "docs")
                require(blocks == re.findall(r"```hcl\n(.*?)\n```", other.read_text(), re.S),
                        f"Translated HCL example differs: {label}")
        for locale in ("en", "ko"):
            target = ROOT / "docs" / ("ko/guides" if locale == "ko" else "guides") / "schema_reference.md"
            expected = schema_reference(schema, locale)
            if write_reference:
                target.write_text(expected)
            verify_reference(target.read_text(), expected, str(target.relative_to(ROOT)))
    print(f"{len(documents)} provider-page HCL examples passed format/validate and translation checks; complete bilingual runtime schema references match.")
    print("Narrative semantics, validators/plan modifiers, Registry rendering and live API behavior remain separate checks.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--terraform", type=Path, required=True, help="Actual Terraform executable, not a home-dependent wrapper")
    parser.add_argument("--provider-dir", type=Path, required=True, help="Directory containing the built provider binary")
    parser.add_argument("--write-schema-reference", action="store_true", help="Regenerate only the two schema reference pages after HCL checks")
    args = parser.parse_args()
    try:
        check(str(args.terraform.resolve()), args.provider_dir.resolve(), args.write_schema_reference)
    except (OSError, KeyError, ValueError, subprocess.SubprocessError) as error:
        # Do not dump subprocess output or inherited environment values.
        detail = str(error) if type(error) is ValueError else type(error).__name__
        parser.exit(1, f"Introductory documentation validation failed: {detail}\n")


if __name__ == "__main__":
    main()
