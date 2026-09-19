#!/usr/bin/env python3
"""Offline consistency checks. This is not provider/API acceptance testing."""
import json
import pathlib
import re
import sys
import urllib.parse

ROOT = pathlib.Path(__file__).resolve().parents[1]
errors = []


def require(condition, message):
    if not condition:
        errors.append(message)


ko = ROOT / "design/ko"
en = ROOT / "design/en"
require({p.name for p in ko.glob("*.md")} == {p.name for p in en.glob("*.md")},
        "Korean and English document sets differ")
for file in ko.glob("*.md"):
    other = en / file.name
    if other.exists():
        for prefix, width in [("C", 2), ("T", 3)]:
            regex = rf"(?<![A-Za-z0-9]){prefix}\d{{{width}}}(?![A-Za-z0-9])"
            require(set(re.findall(regex, file.read_text())) == set(re.findall(regex, other.read_text())),
                    f"Contract/check IDs differ: {file.name}")

provider_docs = ROOT / "docs"
provider_ko = provider_docs / "ko"
english_provider_paths = {p.relative_to(provider_docs) for section in ("resources", "guides")
                          for p in (provider_docs / section).glob("*.md")}
korean_provider_paths = {p.relative_to(provider_ko) for section in ("resources", "guides")
                         for p in (provider_ko / section).glob("*.md")}
require(english_provider_paths == korean_provider_paths, "Provider resource/guide translations differ")
for relative in english_provider_paths & korean_provider_paths:
    en_text = (provider_docs / relative).read_text()
    ko_text = (provider_ko / relative).read_text()
    require(set(re.findall(r"(?<![A-Za-z0-9])(?:C\d{2}|T\d{3})(?![A-Za-z0-9])", en_text)) ==
            set(re.findall(r"(?<![A-Za-z0-9])(?:C\d{2}|T\d{3})(?![A-Za-z0-9])", ko_text)),
            f"Provider document contract/check IDs differ: {relative}")

for file in ROOT.rglob("*.md"):
    if ".git" in file.parts:
        continue
    content = file.read_text()
    require(content.count("```") % 2 == 0, f"Unbalanced code fence: {file.relative_to(ROOT)}")
    for target in re.findall(r"\[[^\]]*\]\(([^)]+)\)", content):
        url = urllib.parse.urlsplit(target)
        if url.scheme or target.startswith("#"):
            continue
        path = urllib.parse.unquote(url.path)
        if path:
            require((file.parent / path).is_file(), f"Missing local link: {file.relative_to(ROOT)} -> {target}")

inventory = json.loads((ROOT / "design/inventory/api.json").read_text())
operations = []
for entry in inventory["entries"]:
    require(entry["source"].startswith("https://"), "Source must be HTTPS")
    require("fetch_error" not in entry, f"Unresolved fetch failure: {entry['source']}")
    for operation in entry["operations"]:
        operations.append((operation["method"], operation["path"]))
require(len(operations) == len(set(operations)), "Duplicate API method/path")
require(len(operations) > 0, "Empty API inventory")
families = {e["family"] for e in inventory["entries"]}
require(families == {"iaas", "common", "hosting", "cache", "dbms", "nas", "webmail"}, "Missing API family")

ledger = json.loads((ROOT / "design/inventory/implementation.json").read_text())["capabilities"]
implemented = {}
for capability in ledger:
    require(capability["ownership"] and capability["import"], "Missing lifecycle decision")
    require(capability["evidence"], "Missing implementation evidence")
    for evidence in capability["evidence"]:
        require((ROOT / evidence).is_file(), f"Missing implementation evidence: {evidence}")
    for operation in capability["operations"]:
        key = (operation["method"], operation["path"])
        require(key in operations, f"Implemented operation absent from discovery: {key}")
        implemented[key] = capability
# Internal adapters are evidence, not registered Terraform capabilities. Keep
# their operations out of the public implementation/live flags above.
adapters = json.loads((ROOT / "design/inventory/implementation.json").read_text()).get("internal_adapters", [])
require(len({a["name"] for a in adapters}) == len(adapters), "Duplicate internal adapter")
for adapter in adapters:
    require(adapter["terraform_registered"] is False, "Internal adapter claimed Terraform registration")
    require(adapter["ownership"] and adapter["import"], "Missing internal adapter lifecycle decision")
    require(bool(adapter["evidence"]), "Missing internal adapter evidence")
    for evidence in adapter["evidence"]:
        require((ROOT / evidence).is_file(), f"Missing adapter evidence: {evidence}")
    for operation in adapter["operations"]:
        require((operation["method"], operation["path"]) in operations, "Adapter operation absent from discovery")
for entry in inventory["entries"]:
    matches = [implemented.get((op["method"], op["path"])) for op in entry["operations"]]
    expected = bool(matches) and all(matches)
    require(entry["implemented"] == expected, f"Implementation flag drift: {entry['source']}")
    require(entry["live_verified"] == (expected and all(c["live_verified"] for c in matches)),
            f"Live verification flag drift: {entry['source']}")

cli = json.loads((ROOT / "design/inventory/cli.json").read_text())["entries"]
require(len({c["command"] for c in cli}) == len(cli), "Duplicate CLI command")
require(all(c["exit_code"] == 0 for c in cli), "CLI discovery failed")
service_operations = json.loads((ROOT / "design/inventory/service-operations.json").read_text())["entries"]
require(len({(op["family"], op["operation"]) for op in service_operations}) == len(service_operations),
        "Duplicate service operation")
require(all(op["source"].startswith("https://") for op in service_operations), "Invalid service operation source")

surfaces = json.loads((ROOT / "design/inventory/surfaces.json").read_text())["entries"]
for entry in surfaces:
    require("fetch_error" not in entry, f"Unresolved surface fetch: {entry['source']}")
checks = json.loads((ROOT / "design/inventory/checks.json").read_text())["checks"]
require(len({c["id"] for c in checks}) == len(checks), "Duplicate test IDs")
for check in checks:
    require(check["status"] in {"not_run", "in_progress", "passed", "failed", "blocked"}, f"Invalid status: {check['id']}")
    require(bool(check["expected_en"] and check["expected_ko"]), f"Missing translation: {check['id']}")
    if check["status"] == "passed":
        require(bool(check["evidence"]), f"Passing check lacks evidence: {check['id']}")
    for evidence in check["evidence"]:
        require((ROOT / evidence).is_file(), f"Missing evidence: {check['id']} -> {evidence}")
    for language in ("en", "ko"):
        row = (f"| {check['id']} | {check['phase']} | {check['method']} | "
               f"{check['expected_' + language]} | {check['status']} |")
        require(row in (ROOT / f"design/{language}/verification.md").read_text(),
                f"Checklist drift: {language}/{check['id']}")

if errors:
    print("\n".join(errors), file=sys.stderr)
    sys.exit(1)
print(f"Documentation checks passed: {len(list(ko.glob('*.md')))} design language pairs, "
      f"{len(english_provider_paths)} provider language pairs, "
      f"{len(operations)} API operations, {len(surfaces)} surfaces, {len(checks)} planned checks.")
print("This documentation check does not execute API or provider acceptance tests.")
