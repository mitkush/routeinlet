# AI subscription gateway

PRODUCT REQUIREMENTS DOCUMENT     ·     REVIEW DRAFT     ·     12 NOVEMBER 2026

A developer should be able to connect an eligible AI subscription to a project, run a real product feature, and decide whether metered API access is worth buying. The gateway should show what reached the model and which account funded each request.

## Product decision

Ship a self-hosted HTTP service with a command-line interface, native executables, and a container image. The project owner configures the upstream connection once. The application keeps its own users, permissions, and interface. An optional admin UI follows the first release.

| Decision | Product requirement |
| --- | --- |
| Ownership | Each project controls its connections, quota policy, fallback credentials, and access to its gateway. |
| Portability | Any language that can call HTTP can use the service. Support workstations, CI, and server deployments with the same request contract. |
| API coverage | Expose verified operations on each connection. Preserve native behavior and identify unsupported or unverified features explicitly. |
| Spending | Start with subscription usage. When the configured quota policy activates a paid route, continue automatically and notify the calling application. |
| Testing | Preserve inputs by default. Developers opt into token-saving transformations and can measure their effect on task quality. |

## Decision still open

The first provider, pilot application, and implementation language remain open. The launch dependency is a subscription connection whose supported operations and permitted deployment match the pilot. A working local login alone does not establish project-wide production eligibility.

## Developer experience

The primary user is a developer adding AI to an existing application. The operator is whoever maintains that project’s deployment and credentials. One person may perform both roles.

| Step | Developer experience | Evidence of success |
| --- | --- | --- |
| Install | Download an executable or run the published container image. | The same version reports readiness on a laptop and in a headless environment. |
| Connect | Select a project, authorize an eligible plan, and inspect supported models and operations. | The CLI reports owner scope, connection health, deployment profile, and verified capabilities. |
| Integrate | Point the app’s backend at the gateway using an HTTP client or compatible SDK. | A real feature returns its expected provider response format, with a traceable request ID. |
| Evaluate | Run representative inputs and existing app assertions. Enable optimizations only when desired. | The owner can compare task success, latency, and measured or estimated usage. |
| Continue | Configure a paid API credential and select quota exhaustion as a fallback trigger. | The app receives the expected result and a warning when the paid route serves it. |

### Illustrative CLI

```bash
ai-gateway connect --project demo
ai-gateway capabilities --project demo
ai-gateway start --project demo
```

Names and flags are proposed. Setup must work without a gateway dashboard. A remote operator can complete any required provider authorization through a supported browser flow on a separate device.

### The first five minutes

After installation, a developer can start the service, send a fixture request, and inspect its route without purchasing API credits. A fixture exercises setup only. The CLI labels live provider responses separately so the developer knows when a feature has actually been tested.

## Request compatibility

The gateway makes routing decisions at the level of operation, model, transport, and connection. An account with access to a model may expose a different set of operations through its subscription.

| ID | Required behavior | Acceptance example |
| --- | --- | --- |
| R1 | Publish a versioned capability inventory with evidence and last verification date. Distinguish supported, unsupported, and unknown. | The app can discover streaming, schema enforcement, tool use, and image input independently. |
| R2 | Offer a preflight check against the actual request body. Return the chosen route, field paths that conflict, and any required changes. | A request requiring vision on a text-only route identifies its image field before an inference call. |
| R3 | Preserve input, attachments, tool definitions, required schema, and stream semantics by default. Reject a known incompatible route before dispatch. | An invoice image reaches a vision-capable model intact. If every eligible route lacks vision, return a compatibility error. |
| R4 | Pass provider-specific extensions through on native routes. Report unknown compatibility where evidence is incomplete. In strict evaluation mode, require verification for declared mandatory features. | A new optional native field reaches its provider. Unknown support for a mandatory field is visible to the caller. |
| R5 | Expose additional operations as adapters verify them. Track text, files, images, audio, embeddings, batch, and stateful operations separately. | Discovery lists each operation with its supported transport and limitations. An unsupported call returns a typed error. |

### Developer facing API

Proposed routes include GET /gateway/v1/capabilities and POST /gateway/v1/preflight. Provider operations retain their adapter-documented native paths, request bodies, status codes, and event formats. Preflight is local and advisory. Dispatch rechecks compatibility and eligibility because connection state can change.

## Quota and paid fallback

A project can register a subscription and a metered route. The owner defines when the second route may run. A configured quota-triggered fallback proceeds automatically and gives the app a warning it can display.

| Observed condition | Gateway behavior |
| --- | --- |
| Quota exhausted | Use a configured, compatible fallback within its spend limit. Attach request ID, billing source, actual model, and reason. Without fallback, return quota_exhausted and the reset time if known. |
| Transient throttling | Honor provider retry timing within the request deadline. A generic HTTP 429 alone does not establish that a weekly allowance is exhausted. Report an unknown cause when necessary. |
| Model or feature changes | Evaluate the backup against the request. Use a different model only when the project allows it. Preserve required features or return a compatibility error. |
| Partial stream or unknown outcome | After sending model output to the app, surface a terminal error if the stream breaks. Avoid replay when an upstream operation might already have executed. |
| Concurrent spend | Where a cap is configured, reserve a conservative allowance atomically before paid dispatch. Reconcile against reported usage. Decline paid dispatch when the cap cannot be enforced. |

### Warning the app can display

```text
Gateway-Request-Id: req_42
Gateway-Billing-Source: metered_api
Gateway-Fallback-Reason: subscription_quota_exhausted
```

Example app message: “Your subscription allowance was reached. This request used the configured paid API.” Send route metadata in HTTP headers before the first output event. Keep the provider’s JSON and streaming payload intact. Record the same transition for background jobs and charge estimates after completion.

Treat reported resets, token counts, and prices as independent facts. A provider may expose a quota window without a token balance. Show unavailable values as unknown and label estimates with the price version.

## Optimization and evaluation

Token savings matter when the application still completes its task. The owner should be able to demonstrate this against their own inputs before enabling an optimization in a shared environment.

| Mode | Behavior and safeguards |
| --- | --- |
| Baseline | Preserve prompts and attachments. Disable gateway response reuse, request merging, and lossy compression by default. Record whether provider-side caching is observable. |
| Opt-in reuse | Cache or merge read-only requests only when the app supplies an appropriate scope and freshness policy. Include model, route, input, relevant settings and tool configuration in the key. Honor the app’s authorization boundary. |
| Opt-in compression | Show which fields would change and allow protected fields. Never remove a required file, schema, or tool. Keep the original available to the caller for comparison. |
| Paired evaluation | Run baseline and candidate configurations over the same labeled examples. Record model/version differences, repeated-run variability, timeouts, invalid outputs, and missing usage. Disable side effects during replay. |

### Report that answers the purchasing question

For an invoice extractor, the developer supplies a fixture set and checks total, currency, and valid JSON. Report passes / attempts, latency distribution, provider-reported usage, and priced API-equivalent cost per passing result. Divide total estimated spend for all attempts, including failures and retries, by passing results. If none pass, report the value as undefined.

A subscription trial can establish that a workflow works on that connection. The report separates measured subscription results, estimated metered costs, and any later observed API results. It identifies fields or operations that still require a paid-route test.

### Delivery priority

The first release preserves a baseline and exports per-request evidence. A subsequent release adds opt-in reuse, compression, and paired evaluation reports. The project chooses the quality threshold and may disable an optimization per request.

## Deployment and operations

A single service owns connection setup, project authentication, preflight, routing, and response metadata. An adapter owns provider-native authentication, protocol translation, and quota interpretation. The host app owns user identity, application permissions, and presentation of gateway messages.

| Area | Release requirement |
| --- | --- |
| Distribution | Publish versioned native builds for macOS, Windows, and Linux and a Linux container image. Validate the named OS and architecture combinations before advertising support. |
| Service lifecycle | Provide readiness and liveness checks, graceful shutdown, configuration validation, and durable connection state across restarts. Expired credentials produce a reauthorization action. |
| Secrets and isolation | Keep provider credentials server-side. Use OS credential storage or deployment secrets. Separate project access keys and admin rights. Isolate metering, caches, and configuration by project. |
| Data handling | Log request ID, route, model, billing source, status, and timing. Omit raw prompts, files, outputs, and credentials by default. Allow bounded, explicit capture for evaluation with a deletion control. |
| Diagnostics | Separate gateway errors from upstream failures. Include request ID and retry guidance. Honor cancellation, bound concurrent work, and distinguish measured cost from estimates. |
| Optional owner UI | After validating the CLI flow, expose connections, capabilities, fallback policy, spend, and request history in a small admin interface using the same service API. |

### Proposed engineering targets

Measure p95 gateway overhead on a fixed synthetic upstream at 20 concurrent clients. Initial target: at most 50 ms of added time to first byte with optimization off. Record hardware, payload sizes, streaming mode, and direct-upstream baseline. Publish measured results before using performance claims.

## Release plan and decision gates

| Stage | Deliverable | Exit evidence |
| --- | --- | --- |
| Feasibility | Select the pilot workflow and verify one provider plan for its deployment and audience. | Document owner scope and supported operations. Demonstrate one authorized live feature end to end. |
| Private alpha | Ship CLI, executable, container, native HTTP route, discovery, preflight, request metadata, and quota-triggered fallback. | Exercise laptop and headless deployments. Validate normal, incompatible, and exhausted-quota requests with fixtures and an eligible live route. |
| Public beta | Add opt-in optimization and paired evaluation. Publish the tested capability and deployment matrix. | Two independent projects using different client languages complete integration. Compare task outcomes on owner-supplied examples. |
| Broader release | Expand adapters and operations with evidence. Add the dashboard after observing common operator tasks. | Each newly advertised connector passes its own compatibility, quota, and billing tests for the deployment claimed. |

### Proposed launch scorecard

Activation: at least four of five pilot developers complete their first live request within 15 minutes after installing with an eligible account. Measure via pilot observation or opt-in reporting. Compatibility: every known incompatible mandatory field in the release fixtures is flagged before inference. Billing: every configured paid fallback carries a caller-visible warning, including a streaming case.

Reliability fixtures cover an expired login, a temporary 429, exhausted quota with and without paid backup, concurrent requests at a spend cap, client cancellation, an interrupted stream, and a request whose upstream outcome is unknown. Release evidence distinguishes simulated quota transitions from provider-observed transitions.

### Open decisions

Product owner selects the first provider, pilot project, and quality checks after feasibility review. Engineering proposes the runtime, exact API names, secret-store support, and tested CPU architectures. Numeric targets above are review proposals. Release dates depend on connector feasibility and pilot results.

## Evidence and launch dependencies

The following findings shaped the requirements. Repository evidence covers specific code paths. Provider eligibility must be checked for the actual plan, audience, and deployment before publishing a connector as supported.

### Existing gateways

9Router can retain less-capable models in its fallback list and replace unsupported media during translation. An isolated helper check preserved an image for a vision-capable target and replaced it for a text-only target. This motivates required-input checks on every candidate route.

[9Router source  ·  Fallback ordering](https://github.com/decolua/9router/blob/54a88d0498ffea4e52576950235562ea420dc761/open-sse/services/combo.js#L84)

[9Router source  ·  Media handling](https://github.com/decolua/9router/blob/54a88d0498ffea4e52576950235562ea420dc761/open-sse/handlers/chatCore.js#L182)

OmniRoute already rejects certain unsupported tool requests and exposes account-fallback metadata. Its tool guard was exercised in isolation. Generic fallback counts and capability checks are established features. The proposed billing warning identifies the funding change for an individual request.

[OmniRoute source  ·  Tool guard and caller feedback](https://github.com/cheaper-inference/OmniRoute/blob/5a5894a77604e3148b0f76b14d8b0d9e4411edcb/open-sse/handlers/chatCore/toolCallingRequiredCheck.ts#L19)

[OmniRoute source  ·  Fallback metadata](https://github.com/cheaper-inference/OmniRoute/blob/5a5894a77604e3148b0f76b14d8b0d9e4411edcb/src/sse/handlers/chat/selectedConnectionHeader.ts#L43)

CLIProxyAPI removes specific fields in its xAI Responses path. Its WebSocket handling distinguishes an unsent operation from a disconnected turn with an unknown outcome. Both cases inform compatibility and retry requirements.

[CLIProxyAPI source  ·  Request shaping](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/internal/runtime/executor/xai_executor_request.go#L131)

[CLIProxyAPI source  ·  Stream interruption](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/sdk/cliproxy/executor/websocket.go#L33)

### Provider feasibility

OpenAI documents eligible requests against a consenting user’s plan for open-source and locally hosted apps and directs paid or remotely hosted apps to an interest process. Anthropic restricts third-party routing on behalf of users through consumer Claude plan credentials. Cursor and Grok remain candidates pending provider-specific verification. No project-wide consumer plan has been validated for launch.

[OpenAI  ·  ChatGPT plan usage and deployment eligibility](https://developers.openai.com/siwc/token-sharing-open-source)

[Anthropic  ·  Claude Code authentication and credential use](https://code.claude.com/docs/en/legal-and-compliance)
