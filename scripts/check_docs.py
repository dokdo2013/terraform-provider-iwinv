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
            regex = rf"\b{prefix}\d{{{width}}}\b"
            require(set(re.findall(regex, file.read_text())) == set(re.findall(regex, other.read_text())),
                    f"Contract/check IDs differ: {file.name}")

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

surfaces = json.loads((ROOT / "design/inventory/surfaces.json").read_text())["entries"]
for entry in surfaces:
    require("fetch_error" not in entry, f"Unresolved surface fetch: {entry['source']}")
checks = json.loads((ROOT / "design/inventory/checks.json").read_text())["checks"]
require(len({c["id"] for c in checks}) == len(checks), "Duplicate test IDs")
for check in checks:
    require(check["status"] in {"not_run", "passed", "failed", "blocked"}, f"Invalid status: {check['id']}")
    require(bool(check["expected_en"] and check["expected_ko"]), f"Missing translation: {check['id']}")
    if check["status"] == "passed":
        require(bool(check["evidence"]), f"Passing check lacks evidence: {check['id']}")
    for language in ("en", "ko"):
        row = (f"| {check['id']} | {check['phase']} | {check['method']} | "
               f"{check['expected_' + language]} | {check['status']} |")
        require(row in (ROOT / f"design/{language}/verification.md").read_text(),
                f"Checklist drift: {language}/{check['id']}")

if errors:
    print("\n".join(errors), file=sys.stderr)
    sys.exit(1)
print(f"Documentation checks passed: {len(list(ko.glob('*.md')))} language pairs, "
      f"{len(operations)} API operations, {len(surfaces)} surfaces, {len(checks)} planned checks.")
print("No authenticated API or provider acceptance tests were performed.")
