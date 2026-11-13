# RouteInlet AI subscription gateway

PRODUCT REQUIREMENTS DOCUMENT     ·     REVIEW DRAFT     ·     13 NOVEMBER 2026

> **Use the AI subscriptions you already pay for in your applications. Spend on paid APIs only when you enable fallback—and always know when that happens.**

This review draft defines the planned behavior. The guarantee covers paid dispatches that RouteInlet controls. If a provider can charge overage on the same credential, the gateway must identify that exposure before use. Only routes with enforceable included-only usage qualify for a strict subscription-only policy.

## Product decision

Ship a self-hosted HTTP service with a command-line interface, native executables, and a container image. The project owner configures the upstream connection once. The application keeps its own users, permissions, and interface. An optional admin UI follows the first release.

| Decision | Product requirement |
| --- | --- |
| Ownership | Each project controls its connections, quota policy, fallback credentials, and access to its gateway. |
| Portability | Any language that can call HTTP can use the service. Support workstations, CI, and server deployments with the same request contract. |
| API coverage | Expose verified operations on each connection. Preserve native behavior and identify unsupported or unverified features explicitly. |
| Spending | Start with subscription usage. Before automatic paid dispatch, enforce the owner's trigger and spend cap and obtain delivery acknowledgment from the configured receiver. |
| Testing | Preserve inputs by default. Developers opt into token-saving transformations and can measure their effect on task quality. |

## Target subscriptions and pilot scope

Investigate all four plans in the order below. Existing connectors, authenticated client execution, and undocumented protocols are in scope for technical investigation. An official integration is not a prerequisite for investigation. Record technical findings separately from the provider terms and deployment evidence needed to advertise support. Live account tests remain outstanding for every row.

| Order | Exact target plan | Investigation and deployment question |
| --- | --- | --- |
| 1 | Claude Pro · $20/month | Evaluate CLIProxyAPI’s Claude transport and authenticated native client execution. Anthropic restricts third-party credential collection and routing on behalf of end users. Record the audience and deployment each finding covers. |
| 2 | ChatGPT Plus · ₹1,999/month target in India | Evaluate CLIProxyAPI’s ChatGPT transport and the documented plan-usage consent flow. Verify live inference and allowance evidence for the authorized owner and deployment. |
| 3 | Cursor Plus · $8/month | Evaluate 9Router’s Cursor adapter with an actual Plus account. Verify account-visible first-party models, allowance evidence, and supported deployment. |
| 4 | Cursor Premium · $30/month | Repeat qualification on Premium. Test included first-party usage separately from third-party models billed from prepaid credit. |

### Milestone one: subscription connectivity

Maintain a dated evidence record per plan, account tier, client/connector version, and deployment. Advance each plan through **authentication → completed inference → included-usage evidence → application-feature validation**. Record unknown funding when the provider supplies insufficient evidence. A valid credential or a completed response alone cannot establish included usage.

| Check | Required evidence |
| --- | --- |
| Real application request | A reference app sends the request and validates the result against a feature-specific assertion. Link the request ID to its upstream outcome. |
| Streaming and conversation history | The app receives ordered native stream events and completion. A follow-up question correctly uses a fact supplied only in the previous turn. |
| Account model access | Compare advertised models with those available to the connected account. Exercise the selected model and report the actual model when observable. |
| Expiry and renewal | Observe renewal or required reauthorization, recovery after restart, and an actionable diagnostic when renewal fails. |
| Quota and funding | Distinguish exhausted allowance, transient throttling, and unknown causes. Record provider evidence for included or additional charges. Label fixtures separately from live observations. |

### Alpha scope and transport selection

Start with one project and one gateway process. Include subscription connections, request preflight, native streaming, conversation continuity, request history, and optional notified paid fallback. Defer compression, semantic caching, a large dashboard, distributed replicas, and broad protocol emulation until the baseline passes the release gates.

Evaluate CLIProxyAPI for Claude and ChatGPT and 9Router for Cursor. Pin the engine version and verify that internal fallback, retries, and request transformations can be disabled or exposed to RouteInlet's policy before execution. Reject a transport for a route if it can silently substitute a model, replay an operation, alter required inputs, or trigger spend outside that policy.

### Application reliability benchmark

| Promise | Acceptance criterion |
| --- | --- |
| Preserve the feature | Images, instructions, tool definitions, conversation history, and required output schemas survive routing intact. |
| Explain incompatibility | Preflight identifies the conflicting field before inference, including on alternative routes. |
| Make funding visible | Each request records the selected connection, actual model or unknown, funding evidence, and fallback reason. |
| Recover predictably | Expiry, renewal failures, and upstream protocol changes produce diagnostics with a practical next action. |
| Prevent surprise spending | A paid attempt has a matching policy decision, enforceable reservation, and acknowledged notice. |

Run the same workload against pinned versions of RouteInlet and comparison gateways. Record configuration, model, fixtures, expected behavior, and observed results. Publish a comparative claim only for workloads and deployments tested.

## Developer experience

The primary user is a developer adding AI to an existing application. The operator is whoever maintains that project’s deployment and credentials. One person may perform both roles.

| Step | Developer experience | Evidence of success |
| --- | --- | --- |
| Install | Download an executable or run the published container image. | The same version reports readiness on a laptop and in a headless environment. |
| Connect | Select a project, authorize an eligible plan, and inspect supported models and operations. | The CLI reports owner scope, connection health, deployment profile, and verified capabilities. |
| Integrate | Point the app’s backend at the gateway using an HTTP client or compatible SDK. | A real feature returns its expected provider response format, with a traceable request ID. |
| Evaluate | Run representative inputs and existing app assertions. Capture baseline results for later optimization comparisons. | The owner can compare task success, latency, and measured or estimated usage. |
| Continue | Configure paid routes, allowed triggers, a spending limit, and the application’s notification receiver once. | The reference app displays the transition notice and its receiver acknowledges delivery before paid dispatch. |

### Illustrative CLI

```bash
routeinlet connect --project demo
routeinlet capabilities --project demo
routeinlet start --project demo
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

**Routing order:** requested subscription → owner-approved compatible subscription alternatives → configured paid API → clear error.

Recheck compatibility at each step. The owner policy must authorize any model or provider substitution. Exhausting the subscription candidates does not itself authorize spending.

| Condition | Required action |
| --- | --- |
| Confirmed quota exhaustion | Try approved compatible subscription alternatives in order. Enter paid fallback only for an explicitly allowed trigger. Otherwise return the reason and reset time if known. |
| Generic HTTP 429, expired login, unexplained upstream failure | Classify the cause if evidence permits. Honor retry timing within the deadline or return reauthorization or diagnostic guidance. None of these signals alone activates paid fallback. |
| Incompatible candidate | Record the conflicting field and skip the candidate. Return a typed error if no eligible route remains. |
| Partial output or uncertain execution | Once output reaches the app, terminate the stream with an explicit error on failure. When an upstream execution outcome is unknown, surface that state and stop automatic replay. |

### Automatic paid fallback and delivery guarantee

The owner configures the allowed routes, triggers, enforceable spend limits, and notification receiver once. A receiver acknowledgment is automatic and requires no per-request human approval.

1. Verify the trigger, route, actual request requirements, and an enforceable upper bound on the charge. Atomically reserve that maximum against the project budget. Stop if the bound cannot be enforced.
2. Persist the pending funding change, reservation, and a stable transition ID before contacting the receiver.
3. Deliver a notice identifying the request, proposed paid route, model, trigger, and reserved amount. The receiver acknowledges the same transition ID after accepting the notice for delivery. Deduplicate notification retries using that ID.
4. Persist the acknowledgment before sending the paid request. On delivery failure, timeout, or invalid acknowledgment, stop paid dispatch and return a notification error. Release the reservation once the gateway establishes the request was never dispatched.
5. Record the dispatch and its outcome. Reconcile against provider-reported charges when available. After a crash with uncertain dispatch or final usage, retain the reservation for reconciliation and surface the uncertainty.

The reference application must display the notice in its interaction history, including for streaming requests. Test receiver delivery and visible display separately. Response headers carry the same transition ID and billing metadata without altering the model response.

```text
Gateway-Request-Id: req_42
Gateway-Transition-Id: transition_42
Gateway-Billing-Source: metered_api
Gateway-Fallback-Reason: subscription_quota_exhausted
```

### Provider-side overage and accounting

Track three independent facts: the connection used, the allocation of usage (included, paid, mixed, or unknown), and charges (provider-reported, estimated with price version, or unavailable). A subscription credential may consume a paid balance internally without a gateway fallback. Record that as provider-side allocation. Keep unknown allocation unknown; a successful response, token count, or credential type alone supplies insufficient evidence.

Under strict subscription-only policy, block a route if provider-side charges cannot be disabled or bounded at zero. Surface the reason in preflight. Persist reservations across restarts and concurrent requests. Missing final usage must never be treated as zero cost.

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

| Gate | Exit requirement |
| --- | --- |
| Connectivity | One exact plan completes the chosen application task through RouteInlet. Record authentication, inference, funding evidence or unknown, and feature results separately. |
| Fidelity | Required inputs, native streaming, conversation history, and relevant tools survive intact. An incompatible fixture reports its field before inference. |
| Billing | Every paid dispatch correlates to an owner policy, durable reservation, and acknowledged notice. Receiver failure yields zero paid dispatches. Demonstrate the notice in the reference app. |
| Reliability | Pass quota, renewal, cancellation, partial-stream, concurrency, and crash fixtures. Exercise restart both before acknowledgment and after a potentially billable dispatch. Verify uncertain outcomes cannot cause replay or release reserved spend prematurely. |
| Expansion | Each additional plan and deployment passes its own qualification checks. Record provider evidence for the intended audience. |

Release progression depends on gate evidence. Report simulated quota and crash behavior separately from live provider observations. As a proposed usability target, at least four of five pilot developers should complete a first live request within 15 minutes of installation with an eligible account.

### Open product choices

1. **First real application feature.** The provisional recommendation is a streaming text assistant with a multi-turn history check. The product owner will choose the pilot. Document extraction and tool-using applications follow as capability tests.
2. **Initial setup experience.** The provisional recommendation is local CLI setup. Validate first-run connection and diagnostics with developers before finalizing onboarding. Retain headless and container deployment as platform goals.

Go remains an engineering proposal for the CLI and HTTP service. The runtime and packaging choices remain subject to the transport evaluation.

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

The plan matrix above reflects published provider documentation. Live account tests and confirmation of project-wide consumer-plan eligibility are pending. Capture the plan, deployment, evidence date, and verified operations when changing a connector’s status.

[OpenAI  ·  ChatGPT plan usage and deployment eligibility](https://developers.openai.com/siwc/token-sharing-open-source)

[Anthropic  ·  Claude Code authentication and credential use](https://code.claude.com/docs/en/legal-and-compliance)

[Cursor · Plan allowances and third-party pricing](https://cursor.com/docs/models-and-pricing)

[Cursor and Grok · Included usage and eligible surfaces](https://docs.x.ai/x-subscription/usage-limits)
