# MCP discovery and OAuth audit

[한국어](../ko/mcp-audit.md) · [Coverage](coverage.md) · Observed: 2026-09-19

## Evidence and current gate

C27 and T014 remain incomplete. The [machine-readable record](../inventory/mcp.json) distinguishes public discovery and registration
from authenticated tool discovery. `tools` and `tool_count` are null because no authenticated inventory has been obtained; null does
not mean the server exposes no tools. The Terraform Provider continues to call direct service APIs and has no MCP runtime dependency.

The official connection guide uses `https://mcp.iwinv.kr`. A request to initialize without an access token returned HTTP 401 with a
`mcp:tools` scope challenge and the protected-resource metadata URL. Discovery points to `https://oauth.iwinv.kr`, authorization-code
flow and S256 PKCE. The scope challenge is narrower than the full advertised scope set; the audit requested only `mcp:tools`, without
profile or offline access. The HMAC control-plane key was not supplied to OAuth or used as a bearer token.

Using official MCP Python SDK 2.2.0 in a temporary environment, dynamic registration returned HTTP 201 and provided a registration
management URL and token. The client requested only an authorization-code grant, used a loopback callback with state/issuer validation,
and persisted registration evidence outside Git with mode 0600. No existing Codex/MCP configuration was changed.

The browser reached an iwinv screen that combines login with granting the new client account access. Owner consent was not granted.
The SDK session ended while waiting, without receiving an access token or an authenticated initialize/tools/list result. The exact
nested SDK failure was not retained, so this is not evidence of an iwinv token-endpoint defect. The local listener has stopped.

Cleanup of the **one new, unconsented client registration** remains unverified: DELETE of the returned management URL using the returned
management token produced HTTP 404. A subsequent GET of that same URL returned HTTP 200 and the exact newly registered client ID.
The management token and URL had not rotated. Neither a successful deletion nor a disabled registration can be claimed. No alternate
paths, methods or additional registrations were tried. Resolve registration management cleanup before granting access or creating another
client. There was no MCP tools/call, cloud-resource mutation, mail transmission or DNS change during this audit.

Registration identity, management token, browser authorization URL and raw responses remain in private local evidence, never this
public repository. Do not automatically recreate clients after an observation timeout. Reconcile or reuse the recorded client only
when its lifecycle and authorization conditions are resolved. The separate webmail cleanup failure also remains open.

## Offline capture validation

T076 validates a credential-free preparation tool, **not authenticated MCP acceptance**. `scripts/summarize_mcp.py` consumes ordered,
private SDK capture pages. Each page must include the cursor actually used in its request and the returned SDK result:

```json
{
  "request_cursor": null,
  "response": {
    "tools": [{"name": "example_read", "inputSchema": {"type": "object", "properties": {}}}],
    "nextCursor": null
  }
}
```

Run it only on deliberately captured files, in request order:

```sh
python3 scripts/summarize_mcp.py /absolute/private/page-0.json /absolute/private/page-1.json
python3 -m unittest discover -s scripts -p 'test_summarize_mcp.py' -v
```

It rejects missing/reordered/repeated/incomplete pages, duplicate or malformed tool names, invalid input shapes and invalid boolean hints.
It emits only names, top-level input names/types/required fields, explicit boolean annotations and output-schema presence. Descriptions,
defaults, examples, nested schema content, `_meta` and continuation cursors are not emitted. Errors omit captured values. Empty declared
types mean no direct `type` declaration, not that an argument accepts no values. Boolean property schemas remain explicit true/false. Combinators, constraints and nested schemas still need
review in private source evidence. This summary is not a complete JSON Schema or an operation contract.

Tool/property names can themselves contain private information, so manually review output before publication. Do not infer safety,
idempotency or permission from a tool name or annotation. The script performs no network requests or tool calls and does not write an
inventory file automatically. Synthetic pagination, exclusion and failure tests run in the documentation CI without live credentials.

A later same-day reconciliation of the exact returned management URI still returned HTTP 200 and the same registered client. No new client, grant or token was created. The private support draft now includes the registration cleanup issue; it has not been submitted. See the [execution record](contract-progress.md).

## Remaining work

1. Resolve and verify temporary client registration cleanup.
2. Obtain explicit owner OAuth consent for the concrete scope, then authenticate using the recorded supported flow.
3. Capture the complete tool list with request cursors and compare each operation/input to the API/CLI inventory.
4. Review full input/output schemas privately and publish only manually reviewed, account-independent contracts.
5. Verify authorization/session/registration cleanup. No tool listing alone establishes CRUD or Terraform support.

Sources: [iwinv MCP guide](https://docs.iwinv.kr/developers/mcp/),
[iwinv connection instructions](https://docs.iwinv.kr/developers/mcp/codex),
[resource metadata](https://mcp.iwinv.kr/.well-known/oauth-protected-resource),
[authorization metadata](https://oauth.iwinv.kr/.well-known/oauth-authorization-server),
[MCP authorization specification](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization),
[official Python SDK OAuth guide](https://py.sdk.modelcontextprotocol.io/client/oauth-clients/).
