# Architecture and Terraform user experience

[한국어](../ko/architecture.md) · [Index](../../README.en.md) · Status: proposed · Revision: 1

## Goals and boundaries

Build a community provider covering the full supported iwinv API/CLI surface over time.
One remote durable object maps to one Terraform resource. Read-only information maps to data sources;
one-shot operations to Actions; expiring credentials/URLs to ephemeral resources where appropriate.
Local CLI preferences are not cloud resources. Missing API contracts remain tracked blockers, never silent omissions.

Use Go and Terraform Plugin Framework, starting from the official scaffolding template.
Do not shell out to CLI, call MCP, scrape a console, or use `local-exec` to implement provider operations.
The development baseline is Terraform >=1.14.0 in [ADR-0001](contract-progress.md); its matrix is tested during P1.
Actions and ephemeral features receive explicit compatibility requirements rather than silently raising the baseline.

## AWS-style public interface

| Familiar pattern | Proposed iwinv interface | Actual contract |
| --- | --- | --- |
| `aws_instance` | `iwinv_instance` | One API instance; API batch count fixed to one |
| `instance_type` | `instance_type` | Exact `flavor_id`, not a guessed display-name lookup |
| `availability_zone` | `availability_zone` | Exact API `zone_id`; not an AWS region/AZ hierarchy |
| AMI reference | `image_id` | iwinv image ID; do not use `ami` |
| Key pair reference | `ssh_key_ids` | Set of existing iwinv key IDs; send sorted comma-separated IDs |
| Lookup resources | `iwinv_image`, `iwinv_instance_type`, `iwinv_availability_zones` | Strict documented filters, paginate fully |
| Separate SG rules | `iwinv_security_group_ingress_rule`, `iwinv_security_group_egress_rule` | API direction normalized only after contract verification |
| Separate associations | `iwinv_security_group_attachment` | Group + instance relationship; cardinality must be verified |
| EBS-style storage | `iwinv_block_storage` | Creation requires an instance ID; independent-volume parity is blocked |
| Provider aliases | `provider "iwinv" { alias = "secondary" }` | Separate credentials/client; never share cached account data |
| Standard lifecycle | `for_each`, `import`, `timeouts`, `prevent_destroy` | Terraform semantics; not invented server-side protection |

No fictitious `tags`, `default_tags`, ARN, IAM, VPC, subnet, `region`, or inline `user_data` support.
Use `name` for the remote name. User scripts are existing IDs until a supported write API is found.
Expose one canonical spelling per field, not both `flavor_id` and `instance_type` with ambiguous precedence.
Do not copy the AWS provider's implementation; apply its user-facing conventions to iwinv contracts.

## Example UX (design only, not runnable)

```hcl
terraform {
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" } # proposed; not published
  }
}

provider "iwinv" {} # proposed IWINV_ACCESS_KEY / IWINV_SECRET_KEY environment variables

variable "image_id" { type = string }
variable "instance_type" { type = string }
variable "availability_zone" { type = string }

resource "iwinv_instance" "web" {
  name              = "example-web"
  image_id          = var.image_id
  instance_type     = var.instance_type
  availability_zone = var.availability_zone

  timeouts {
    create = "30m" # user-selected example, not an established default
    delete = "30m"
  }

  lifecycle { prevent_destroy = true }
}

output "server_id" { value = iwinv_instance.web.id }
```

Add version constraints when an actual release exists. No copy-paste apply quickstart before P2 passes.
Examples must explain that `prevent_destroy` is a configuration guard: removing the whole resource block
also removes that guard. It does not protect console/API deletion.

## Instance schema proposal

| Attribute | Terraform shape | Mutation and import policy |
| --- | --- | --- |
| `id` | Computed string | Opaque API ID; stable across reads |
| `name` | Required string | In-place update; Korean byte/character limits need verification |
| `description` | Optional string | Confirm null/empty/clear semantics before selecting defaults |
| `image_id` | Required string | Replacement proposal; never silently rebuild the existing disk |
| `instance_type` | Required string | Resize only after downtime/failure semantics verified; reject changes while unsupported |
| `availability_zone` | Required string | Replacement proposal; no presumed live migration |
| `ssh_key_ids` | Optional set(string) | Create-only proposal; absent Read support must be documented |
| `user_script_id` | Optional string | Existing script only; same create-only/import limitation |
| `status` | Computed string | Preserve service state; do not conflate power and provisioning states |
| `public_ip`, `private_ip` | Computed string | Select default NIC by verified discriminator, never array position |
| `network_interfaces` | Computed collection | Preserve multiple interfaces; exact schema pending |
| `timeouts` | Standard timeout configuration | Context-aware create/update/delete budgets, defaults based on measurements |

No default account password or VNC token in normal Read/state. Request an explicit field mask omitting these.
`Sensitive` masks display but does not prevent state persistence. Credentials belong to provider configuration,
not resource state. Password-based services require a documented state exposure policy and investigation of
write-only arguments before release; do not label ordinary sensitive fields as secret-free.

## State, ownership, and operations

- Import uses immutable IDs, then authoritative Read. Composite IDs for rules/attachments use a documented,
  unambiguous parent/child format. A successful import must converge with matching configuration.
- Unknown/null/empty are distinct. Read refreshes managed fields; do not hide drift with broad
  `UseStateForUnknown`, unconditional diff suppression, or ignored changes.
- Read-only-missing creation inputs cannot be reconstructed from an ID. Keep configured creation inputs
  only under an explicit create-only contract; imported state leaves unknown history absent. Document limitations
  and test configuration after import rather than fabricating empty collections or suppressing replacements.
- Group container, rules, and attachments have one writer each. No inline rules competing with standalone rules.
  Collection replacement APIs (allowlists) should have a single authoritative set owner, not one resource per IP.
- Block storage creation is attached to an instance in the documented contract. Do not publish both a volume
  `instance_id` owner and a competing attachment resource until transfer/detach semantics are proved.
- Reboot, rebuild, cache purge, object copy/move, and message sending are Action candidates, never timestamp-triggered
  fake resources. Actions do not currently update resource state; rebuild must reconcile subsequent Read/configuration
  or be held behind a design gate. No automatic replay of irreversible actions.
- VNC/presigned URLs are ephemeral candidates. Do not put these bearer capabilities in persistent data sources.
- Refresh and data sources never perform mutations, even when the underlying read API uses POST.

## Implementation boundaries

`Terraform schema/lifecycle -> service adapter -> typed HTTP client -> official API`

Proposed packages: `internal/provider`, `internal/services/{compute,storage,network,...}`, and `internal/client`.
Keep auth, encoding, paging, errors, rate limiting, and waiters testable independently. Initially keep the client
in this repository; extract a public Go SDK only when reuse and its compatibility contract justify it.
Control-plane HMAC, S3-compatible signing, messaging authentication, and MCP OAuth are separate protocols.
Provider aliases isolate credentials and clients; documentation must identify which service credential each feature needs.

Use secure HTTPS defaults; do not inherit sample code that disables TLS verification. Endpoint overrides, if exposed,
must be explicit and service-scoped. Never forward credentials across redirects/hosts. Clock injection enables signing tests.
No interactive login or dependency on the official CLI credential file.

## Failures, retries, and asynchronous work

Sign every request attempt using a fresh timestamp. Classify HTTP and business errors together, retaining safe diagnostics.
Retry proven safe reads for transient failures with bounded exponential backoff and jitter; honor valid Retry-After.
Do not retry ambiguous create/send/rebuild success without a verified idempotency or reconciliation mechanism.
If Create returns an ID, preserve it in state as early as the framework lifecycle permits, including partial failures;
test that error paths do not orphan a resource. If no ID returns after uncertain success, stop with reconciliation instructions.

Poll within context deadlines; distinguish pending, active, off, work, error and the documented create example's `building`.
Only confirmed absence removes existing state. An empty page, malformed body, 401/403/429/5xx, or unexpected 202 is not absence.
Creation-time eventual-consistency 404 needs a bounded grace period, not immediate removal.
Delete completes only after absence is established; API acceptance is not completion or proof billing has ended.

Rate-limit requests and polls per credential/client below the documented quota. Multiple processes and aliases can share
an upstream quota, so local limiting alone is insufficient: handle 429 and document aggregate concurrency controls.

## Decision gates

Do not finalize public schemas until [contract gaps](api-contract.md) and [verification gates](verification.md) are resolved.
Record schema changes as ADRs, preserve state with migrations, and release immutable semantic versions.
Official source references and AWS examples are indexed in [sources](../sources.md).

## Security-group service boundary (internal implementation)

The typed network adapter is contract preparation, not a registered Terraform resource.
It maps API `title` to name, `content` to description and `icmp` Y/N to a boolean.
Description responses are HTML-decoded once; names are not decoded or normalized.
Null, empty and omitted descriptions remain distinct. Empty update requests are rejected;
an omitted update leaves the existing description under the tested contract. Create omission remains unresolved.

The eventual group resource will own group attributes only, with an exact `FIREWALL-…` import ID and authoritative detail Read.
Inline rules and instance attachments are excluded from the group model and need their own lifecycle/ownership tests.
A known create ID must be persisted even when validation of the remaining receipt fails.
A successful delete acknowledgement requires subsequent detail absence verification.
API errors never delete state, and import/schema/timeouts/partial-state recovery remain unimplemented Terraform gates.
See the [live and synthetic evidence](contract-progress.md) before implementing those gates.
