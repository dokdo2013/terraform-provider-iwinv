"""Synthetic pagination/privacy checks; no credentials, network or tool calls."""
import copy
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

from summarize_mcp import InventoryError, summarize


def tool(name):
    return {"name": name, "description": "PRIVATE-ACCOUNT-DESCRIPTION", "inputSchema": {
        "type": "object", "properties": {
            "zone": {"type": ["string", "null"], "default": "PRIVATE-ZONE-DEFAULT", "examples": ["PRIVATE-EXAMPLE"]},
            "settings": {"type": "object", "properties": {"nested_private": {"default": "PRIVATE-NESTED"}}},
            "composed": {"anyOf": [{"type": "string"}, {"type": "integer"}]},
        }, "required": ["zone"]}, "annotations": {"readOnlyHint": False, "title": "PRIVATE-TITLE"},
        "outputSchema": {"type": "object", "examples": ["PRIVATE-OUTPUT"]}, "_meta": {"token": "PRIVATE-TOKEN"}}


def page(tools, request=None, next_cursor=None):
    return {"request_cursor": request, "response": {"tools": tools, "nextCursor": next_cursor}}


class InventoryTests(unittest.TestCase):
    def test_complete_sorted_shapes_and_excluded_values(self):
        got = summarize([page([tool("z")], next_cursor="PRIVATE-CURSOR"), page([tool("a")], "PRIVATE-CURSOR")])
        self.assertEqual(got["tool_count"], 2)
        self.assertEqual([x["name"] for x in got["tools"]], ["a", "z"])
        self.assertEqual(got["tools"][0]["inputs"], [
            {"name": "composed", "declared_types": [], "required": False},
            {"name": "settings", "declared_types": ["object"], "required": False},
            {"name": "zone", "declared_types": ["null", "string"], "required": True},
        ])
        self.assertEqual(got["tools"][0]["annotations"], {"readOnlyHint": False})
        self.assertNotIn("PRIVATE", json.dumps(got))
        self.assertNotIn("nested_private", json.dumps(got))

    def test_boolean_property_schemas(self):
        t = tool("a")
        t["inputSchema"]["properties"] = {"anything": True, "forbidden": False}
        t["inputSchema"]["required"] = []
        fields = summarize([page([t])])["tools"][0]["inputs"]
        self.assertEqual([x["boolean_schema"] for x in fields], [True, False])

    def test_cli_late_failure_has_no_partial_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            first, second = root / "first.json", root / "second.json"
            first.write_text(json.dumps(page([tool("a")], next_cursor="PRIVATE-CURSOR")))
            second.write_text(json.dumps(page([tool("b")], request="PRIVATE-WRONG")))
            script = pathlib.Path(__file__).with_name("summarize_mcp.py")
            result = subprocess.run([sys.executable, str(script), str(first), str(second)], capture_output=True, text=True)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout, "")
            self.assertNotIn("PRIVATE", result.stderr)

    def test_empty_and_missing_hints(self):
        self.assertEqual(summarize([page([])])["tools"], [])
        t = tool("a"); del t["annotations"]
        self.assertEqual(summarize([page([t])])["tools"][0]["annotations"], {})

    def test_reject_incomplete_and_malformed_without_values(self):
        valid = [page([tool("a")])]
        cases = [[], [page([], next_cursor="PRIVATE-CURSOR")], [page([], request="PRIVATE-CURSOR")],
                 [page([]), page([])], [page([tool("a"), tool("a")])],
                 [page([], next_cursor="x"), page([], request="x", next_cursor="x")],
                 [{"request_cursor": None, "response": {"error": "PRIVATE-FAILURE", "tools": []}}]]
        for change in (
            lambda t: t.update(name="PRIVATE-INVALID/NAME"),
            lambda t: t.update(inputSchema=None),
            lambda t: t["inputSchema"].update(required=["x", "x"]),
            lambda t: t["inputSchema"]["properties"]["zone"].update(type="PRIVATE-TYPE"),
            lambda t: t.update(annotations={"readOnlyHint": "PRIVATE-NOT-BOOL"}),
        ):
            row = copy.deepcopy(valid); change(row[0]["response"]["tools"][0]); cases.append(row)
        for rows in cases:
            with self.subTest(rows_count=len(rows)):
                with self.assertRaises(InventoryError) as caught:
                    summarize(rows)
                self.assertNotIn("PRIVATE", str(caught.exception))


if __name__ == "__main__":
    unittest.main()
