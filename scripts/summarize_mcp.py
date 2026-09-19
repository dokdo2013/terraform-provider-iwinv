#!/usr/bin/env python3
"""Summarize private MCP tools/list captures offline; never call tools.

Each input file is {"request_cursor": null-or-string, "response": <SDK result>}.
Supply pages in request order. Names and property names still require a human
privacy review before publication; descriptions/defaults/examples are omitted.
This is structural discovery, not validation of a tool's API or side effects.
"""
import argparse
import json
import pathlib
import re
import sys

FLAGS = ("readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint")
JSON_TYPES = {"null", "boolean", "object", "array", "number", "integer", "string"}


class InventoryError(ValueError):
    """Messages deliberately contain no captured values."""


def summarize(pages):
    if not isinstance(pages, list) or not pages or len(pages) > 100:
        raise InventoryError("Expected 1–100 ordered capture pages")
    expected_cursor = None
    cursors, names, result = set(), set(), []
    for index, capture in enumerate(pages):
        if not isinstance(capture, dict) or "request_cursor" not in capture:
            raise InventoryError("Missing captured request cursor")
        if capture["request_cursor"] != expected_cursor:
            raise InventoryError("Capture pages are missing, repeated or out of order")
        page = capture.get("response")
        if not isinstance(page, dict) or "error" in page or not isinstance(page.get("tools"), list):
            raise InventoryError("Expected a successful tools/list result")
        for tool in page["tools"]:
            if not isinstance(tool, dict):
                raise InventoryError("Invalid tool object")
            name = tool.get("name")
            if not isinstance(name, str) or not re.fullmatch(r"[A-Za-z0-9_.-]{1,128}", name) or name in names:
                raise InventoryError("Missing, invalid or duplicate tool name")
            names.add(name)
            schema = tool.get("inputSchema")
            if not isinstance(schema, dict) or schema.get("type") != "object":
                raise InventoryError("Expected an object input schema")
            properties, required = schema.get("properties", {}), schema.get("required", [])
            if not isinstance(properties, dict) or not isinstance(required, list) or any(not isinstance(x, str) for x in required):
                raise InventoryError("Invalid input properties or required fields")
            if len(set(required)) != len(required):
                raise InventoryError("Duplicate required field")
            inputs = []
            for field, definition in sorted(properties.items()):
                if not isinstance(field, str) or not field or not isinstance(definition, (dict, bool)):
                    raise InventoryError("Invalid input field")
                if isinstance(definition, bool):
                    inputs.append({"name": field, "declared_types": [], "required": field in required,
                                   "boolean_schema": definition})
                    continue
                raw_types = definition.get("type")
                kinds = raw_types if isinstance(raw_types, list) else [raw_types] if raw_types is not None else []
                if any(not isinstance(x, str) or x not in JSON_TYPES for x in kinds) or len(kinds) != len(set(kinds)):
                    raise InventoryError("Invalid declared JSON type")
                inputs.append({"name": field, "declared_types": sorted(kinds), "required": field in required})
            hints = tool.get("annotations")
            if hints is not None and not isinstance(hints, dict):
                raise InventoryError("Invalid tool annotations")
            selected_hints = {}
            for flag in FLAGS:
                value = (hints or {}).get(flag)
                if value is not None:
                    if not isinstance(value, bool):
                        raise InventoryError("Invalid boolean tool hint")
                    selected_hints[flag] = value
            result.append({"name": name, "inputs": inputs, "required": sorted(required),
                           "annotations": selected_hints,
                           "output_schema_present": tool.get("outputSchema") is not None})
        next_cursor = page.get("nextCursor")
        if next_cursor is not None:
            if not isinstance(next_cursor, str) or not next_cursor or next_cursor in cursors:
                raise InventoryError("Invalid or repeated continuation cursor")
            cursors.add(next_cursor)
        if index < len(pages) - 1 and next_cursor is None:
            raise InventoryError("Unexpected pages after list completion")
        expected_cursor = next_cursor
    if expected_cursor is not None:
        raise InventoryError("Incomplete tools/list capture")
    return {"schema_version": 1, "page_count": len(pages), "tool_count": len(result),
            "scope": "Top-level input shapes only; not a full JSON Schema or verified API contract. Hints are untrusted metadata, not authorization.",
            "tools": sorted(result, key=lambda row: row["name"])}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("pages", nargs="+", type=pathlib.Path)
    args = parser.parse_args()
    try:
        pages = []
        for file in args.pages:
            if file.stat().st_size > 8 * 1024 * 1024:
                raise InventoryError("Capture page exceeds size limit")
            pages.append(json.loads(file.read_text()))
        result = summarize(pages)
    except (OSError, UnicodeError, json.JSONDecodeError, InventoryError) as error:
        detail = str(error) if isinstance(error, InventoryError) else "Unable to read a valid capture file"
        print(detail, file=sys.stderr)
        return 1
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
