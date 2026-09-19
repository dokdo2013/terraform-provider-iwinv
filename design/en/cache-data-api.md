# Cache data API contract preparation

[한국어](../ko/cache-data-api.md) · [Coverage](coverage.md)

C33 / T079 is **in progress**. The existing `iwinv_content_cache` resource manages the service subscription and referrers. It does not manage images, folders, data API credentials or tokens. No data API capability is registered in the provider.

## Observed bootstrap path

On 2026-09-19, one disposable `cache_lite` service was created through the already verified control-plane contract. Its exact created ID became active in the API and operational in the console. The console's management-page link led to `https://image001.share.cache.iwinv.kr/`; signing in with the newly created account and FTP password opened the content manager. The API key management page showed no existing key and a key-generation button. This verifies one product-to-manager mapping, not every cache product, host or account's eligibility.

No key was generated: action-time approval for creating new security-sensitive access was requested and remains pending. Two credential-free GETs to the documented authorization and capacity routes each returned HTTP 400 with `api`, `apiCode`, numeric `code` and string `result`. These are negative access observations, not authenticated response schemas or successful API operations.

The test service was then deleted using its journaled ID. The API acknowledged deletion and returned an empty cache inventory; a fresh console list without search filters also contained no service row. The initial inventory was empty. No image, folder or data API key was created. Private creation credentials, response bodies and IDs remain outside Git. This successful fixture cleanup does not resolve the separate webmail/OAuth cleanup failure T056.

## Documented surface and unresolved behavior

The [vendor specification](https://help.iwinv.kr/manual/938) describes a separate API-key-to-token exchange, user check, image and folder operations, and capacity lookup. It documents Basic authentication for the key exchange and token fields for other calls. Successful payload examples and parameter tables are not sufficient proof of wire encoding or runtime field types. The operation inventory remains `implemented: false` and `live_verified: false` for all twelve `cache_data` entries.

## Proposed implementation boundaries

- Keep service API credentials and endpoint configuration separate from control-plane HMAC credentials. Never send either credential to the other service. A management login or FTP password is not an API key. Do not infer the API origin from the content delivery domain.
- Bind a service client to an explicit, verified HTTPS origin. Reject cross-origin redirects, retain certificate validation, and isolate client/token state per provider configuration and alias. The single observed manager host is evidence for this fixture, not a universal default.
- Keep the exchanged token in memory and out of diagnostics, resource attributes and fixtures. Verify its expiration and refresh contract before implementing refresh; do not decode an unverified JWT and treat its claims as authoritative.
- Define image/folder ownership from observed remote identity and read-back behavior. Before a resource is registered, establish import, duplicate names, case/path normalization, rename/move, overwrite, deletion and missing-parent behavior. Never adopt a same-named object after an uncertain create.
- Reconcile both HTTP status and business result. Establish size/time/rate limits and pagination. Replay no uncertain upload, rename, move or delete. Capacity units and integer encoding require authenticated evidence before defining a numeric schema.
- Continue using supported service APIs in the provider. The console inspection here is contract research; browser login/key generation must not become provider runtime behavior. No documented remote key-management API was established by this observation.

## Next acceptance work

After the pending key-generation decision, use a fresh disposable fixture, obtain the scoped key and prove account binding with authenticated reads. Record request encoding, response/error types and token lifetime; then test one owned folder and image through create/read/update/import/drift/delete. Include peer preservation, partial upload, invalid path, timeout, expired token and post-revocation rejection. Delete the key and fixture, verify console/API absence, and retain only sanitized public evidence. T079 cannot pass on the current bootstrap and unauthenticated results alone.
