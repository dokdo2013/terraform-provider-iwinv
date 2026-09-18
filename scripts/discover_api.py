#!/usr/bin/env python3
"""Read public API documentation and emit reviewable metadata, never account data.

Run manually, then review the diff. Network discovery is not part of normal CI.
Only endpoint/schema facts and source links are retained; vendor prose is not copied.
"""
import concurrent.futures
import datetime
import json
import pathlib
import re
import urllib.parse
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[1]
HOSTS = {
    "iaas": "iwinv", "common": "iwinv-common", "hosting": "iwinv-hosting",
    "cache": "iwinv-cache", "dbms": "iwinv-dbms", "nas": "iwinv-api-nas",
    "webmail": "iwinv-webmail",
}


def fetch(url):
    request = urllib.request.Request(
        urllib.parse.quote(url, safe=":/?=&%"), headers={"User-Agent": "Mozilla/5.0"}
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        return response.read().decode("utf-8")


def inspect(item):
    family, title, url = item
    row = {"family": family, "title": title, "source": url.removesuffix(".md"),
           "evidence": "unverified", "implemented": False, "live_verified": False,
           "operations": []}
    try:
        page = fetch(url)
        row["evidence"] = "documentation"
        for block in re.findall(r"```json[ \t]*\r?\n(.*?)```", page, re.S):
            spec = json.loads(block)
            if "openapi" not in spec:
                continue
            for path, methods in spec.get("paths", {}).items():
                for method, operation in methods.items():
                    if method not in {"get", "post", "put", "patch", "delete", "head", "options"}:
                        continue
                    content = operation.get("requestBody", {}).get("content", {})
                    row["operations"].append({
                        "method": method.upper(), "path": path,
                        "responses": sorted(operation.get("responses", {})),
                        "response_schema_present": any(
                            media.get("schema")
                            for response in operation.get("responses", {}).values()
                            for media in response.get("content", {}).values()
                        ),
                        "request_content_types": sorted(content),
                        "body_fields": {
                            media: {name: {"type": prop.get("type"),
                                          "required": name in data.get("schema", {}).get("required", []),
                                          "deprecated": prop.get("deprecated", False)}
                                    for name, prop in data.get("schema", {}).get("properties", {}).items()}
                            for media, data in content.items()
                        },
                        "parameters": [{"name": p.get("name"), "in": p.get("in"),
                                        "required": p.get("required", False),
                                        "type": p.get("schema", {}).get("type"),
                                        "default": p.get("schema", {}).get("default")}
                                       for p in operation.get("parameters", [])],
                    })
    except Exception as error:
        row["fetch_error"] = type(error).__name__
    return row


def main():
    items = []
    indexes = []
    for family, host in HOSTS.items():
        url = f"https://{host}.readme.io/llms.txt"
        page = fetch(url)
        indexes.append(url)
        items.extend((family, title, link) for title, link in
                     re.findall(r"^- \[([^\]]+)\]\(([^)]+)\)", page, re.M))
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        rows = list(pool.map(inspect, items))
    result = {"schema_version": 1, "observed_on": datetime.date.today().isoformat(),
              "scope": "Seven official control-plane documentation indexes; not all iwinv services.",
              "indexes": indexes, "entries": rows}
    destination = ROOT / "design" / "inventory" / "api.json"
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(f"{len(rows)} pages; {sum(len(r['operations']) for r in rows)} operations; "
          f"{sum('fetch_error' in r for r in rows)} fetch failures")


if __name__ == "__main__":
    main()
