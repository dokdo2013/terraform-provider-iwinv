#!/usr/bin/env python3
"""Snapshot public CLI command names and service API URL references for review."""
import concurrent.futures
import json
import pathlib
import re
from html.parser import HTMLParser

from discover_api import fetch

ROOT = pathlib.Path(__file__).resolve().parents[1]


class Article(HTMLParser):
    def __init__(self):
        super().__init__()
        self.active = False
        self.parts = []

    def handle_starttag(self, tag, attrs):
        if tag == "article":
            self.active = True

    def handle_endtag(self, tag):
        if tag == "article":
            self.active = False

    def handle_data(self, text):
        if self.active:
            self.parts.append(text)


def inspect(url):
    row = {"source": url, "live_verified": False, "implemented": False}
    try:
        parser = Article()
        parser.feed(fetch(url))
        content = "\n".join(parser.parts)
        command_text = re.sub(r"\b[a-z]+://[^\s]*", "", content)
        row["commands"] = sorted(set(re.findall(
            r"\biwinv(?:[ \t]+[a-z][a-z-]*)+", command_text)))
        row["service_urls"] = sorted(set(re.findall(
            r"https://(?:sms|alimtalk)\.bizservice\.iwinv\.kr/[A-Za-z0-9_./-]+", content)))
        row["evidence"] = "documentation"
    except Exception as error:
        row["evidence"] = "unverified"
        row["fetch_error"] = type(error).__name__
    return row


def main():
    # Discover sidebar links; do not interpret a command as a Terraform resource.
    index = fetch("https://docs.iwinv.kr/developers/cli/")
    links = sorted(set(re.findall(r'href="(/developers/cli/commands/[^"#?]+)', index)))
    urls = ["https://docs.iwinv.kr" + link for link in links]
    urls += ["https://docs.iwinv.kr/developers/api/Message_api/",
             "https://docs.iwinv.kr/developers/api/kakao_api/"]
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        rows = list(pool.map(inspect, urls))
    path = ROOT / "design/inventory/surfaces.json"
    path.write_text(json.dumps({"scope": "Public CLI pages and SMS/Alimtalk URL references; "
                               "URL presence is not a verified HTTP contract.",
                               "entries": rows}, ensure_ascii=False, indent=2) + "\n")
    print(f"{len(rows)} surfaces; {sum('fetch_error' in r for r in rows)} fetch failures")


if __name__ == "__main__":
    main()
