# RouteInlet

A self-hosted AI subscription gateway. The first milestone is an owner-run Claude Pro connectivity investigation.

## Run the pilot

This branch implements the loopback HTTP path and a two-turn reference application. It requires Go 1.22+ and an independently configured local Claude transport. The probe uses Python 3's standard library. RouteInlet does not log in to Claude, collect Claude tokens, or configure provider billing. A successful stream alone cannot identify the balance that paid for it.

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
go build -o routeinlet ./cmd/routeinlet
./routeinlet start
```

4. In another terminal with the same model and gateway key exported, run `python3 examples/probe.py`. It calls `POST http://127.0.0.1:8787/v1/chat/completions` twice. The first turn supplies a fresh reference and expects “OK”. The second includes the exact assistant answer in the ordered history and checks the reference. It rejects a missing finish reason or missing successful stream terminator. It reports request IDs, connector-reported model IDs, text delta counts, and funding status. Use a trusted owner-only workstation.
5. Record connector revision, provider/client version, plan, reported model, both request IDs, task outcome, extra-usage state, and any attributable provider-side usage movement. Record missing evidence as unknown. Check actual account models, expiry/renewal, and quota classification separately. Keep credentials out of reports.

## Current boundary

The gateway accepts the streaming text subset (model, stream, messages with system/user/assistant text). It rejects extra fields before inference, forwards accepted JSON intact, streams the connector's SSE bytes as they arrive, and propagates client cancellation. Requests go to one fixed local connector. Redirects and gateway fallback are disabled. A connector 429 reports an unknown cause. A disconnected stream is never replayed. The gateway always reports funding as unknown in this investigation. Live subscription eligibility, connector-internal transformations and billing attribution are outstanding. See the [product requirements](docs/AI_Subscription_Gateway_PRD.md).
