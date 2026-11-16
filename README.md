# RouteInlet

A self-hosted AI subscription gateway. The first milestone is an owner-run Claude Pro connectivity investigation.

To investigate a ChatGPT Plus account, use the [ChatGPT Plus pilot](docs/ChatGPT_Plus_Pilot.md). It uses the same gateway and application fixtures with a separate Codex OAuth connector profile.

## Run the pilot

This branch implements the loopback HTTP path and a reference application for conversation history and streaming. It requires Go 1.22+ and an independently configured local Claude transport. The probe uses Python 3's standard library. RouteInlet does not log in to Claude, collect Claude tokens, or configure provider billing. A successful stream alone cannot identify the balance that paid for it.

1. On your own pilot machine, check the connected plan and the provider's extra-usage setting. For an included-only test, use an account with provider-side extra spending disabled, an isolated connector with a single Claude connection, and no metered keys or substitute routes. If these conditions cannot be established, skip live inference and record funding as unverified. For an official-client baseline, run Claude Code with your own Pro login and check the same two-turn task there separately.
2. Configure the connector to expose its OpenAI-compatible endpoints on loopback. The transport under investigation is [CLIProxyAPI at 9a8ca1b](https://github.com/router-for-me/CLIProxyAPI/tree/9a8ca1b577ef8573c5ccf108450b93ed57716cc4). Its [sample configuration](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/config.example.yaml) defines the controls. Set `server.host` to `127.0.0.1`, use a dedicated auth directory, configure one local connector access key, set `routing.retry.request-retry` to zero and `upstream.claude.server-side-fallback` to false. Verify the transport's actual attempt count and transformations during qualification. Its OAuth implementation and hosted deployment are unqualified. Review [Anthropic's credential-use documentation](https://code.claude.com/docs/en/legal-and-compliance) for your use case.
3. From the repo root, set the variables below. The connector key authenticates only to your local transport. Choose the exact Claude model ID listed by your connector. Treat that list as a candidate inventory until a live call verifies access.

```bash
export ROUTEINLET_CONNECTOR=http://127.0.0.1:8317
export ROUTEINLET_CONNECTOR_KEY='<local connector access key>'
curl -fsS -H "Authorization: Bearer $ROUTEINLET_CONNECTOR_KEY" "$ROUTEINLET_CONNECTOR/v1/models"
export ROUTEINLET_MODEL='<exact Claude model ID>'
export ROUTEINLET_KEY="$(openssl rand -hex 32)"
go test -race ./...
go build -buildvcs=false -o routeinlet ./cmd/routeinlet
./routeinlet start
```

4. In another terminal with the same model and gateway key exported, run `python3 examples/probe.py`. The probe makes three requests to `POST /v1/chat/completions` on loopback. Two short turns check exact “OK” output and recall of a fresh reference from the ordered history. A separate request asks for a 400-word guide. Each fixture requires text, `finish_reason: "stop"`, and a complete `[DONE]` event. The probe reports truncation and filtering as distinct failures.
5. For the long request, the probe reports application-side arrival times using a monotonic clock. Passing requires at least 1,000 characters and three non-whitespace text events with at least 100 ms between consecutive selected events. A burst of buffered text followed by a delayed terminator is inconclusive. A very fast connector can also be inconclusive. These thresholds measure what this application observed on this run.
6. Record the connector revision, provider/client version, plan, reported model, all three request IDs, history result, text-event count and arrival offsets, extra-usage state, and any attributable provider-side usage movement. Record missing evidence as unknown. Check actual account models, expiry/renewal, and quota classification separately. Keep credentials out of reports.

## Current boundary

The gateway accepts the streaming text subset (model, stream, messages with system/user/assistant text). It rejects extra fields and duplicate JSON keys before inference, forwards accepted JSON intact, streams the connector's SSE bytes as they arrive, and propagates client cancellation. Requests go to one fixed local connector. Redirects and gateway fallback are disabled. A connector 429 reports an unknown cause. After headers are committed, an upstream close before the blank line completing `data: [DONE]` aborts the downstream HTTP stream. A body reader sees a transport error even if it has already received partial text and HTTP 200. The response ends at that `[DONE]` event even if the connector keeps its connection open. A disconnected stream is never replayed. The gateway always reports funding as unknown in this investigation. Live subscription eligibility, connector-internal transformations and billing attribution are outstanding. See the [product requirements](docs/AI_Subscription_Gateway_PRD.md).
