# ChatGPT Plus connectivity pilot

Test one project owner's ChatGPT Plus account through the Codex OAuth transport in CLIProxyAPI. The application sends the same three requests as the Claude pilot. The gateway forwards the accepted Chat Completions JSON to the local connector. CLIProxyAPI handles authentication and translates the upstream response. Record the effect of that translation as part of the test.

**Qualification status:** Live inference, included-allowance funding, quota handling, and renewal are pending. The commands below exercise a local owner-run application. OpenAI also documents an app-specific Sign in with ChatGPT flow with explicit plan-usage consent. This experiment uses CLIProxyAPI's Codex login. Assess hosted, shared-owner deployment separately. [Official OpenAI integration guide](https://developers.openai.com/cookbook/articles/sign-in-with-chatgpt)

## Prepare the two processes

Use the RouteInlet PR branch and a CLIProxyAPI executable built from commit `9a8ca1b577ef8573c5ccf108450b93ed57716cc4`. To build the connector from its checkout at that revision, use its Go toolchain and run `go build -o cli-proxy-api ./cmd/server`. The login flag and config layout are revision-specific. [CLI entry point](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/cmd/server/main.go) · [Configuration reference](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/config.example.yaml)

Use a private local directory outside the checkout. The following shell example assumes macOS or Linux and a free gateway port 8787 and connector port 8318. Stop the previous gateway before switching pilots. Keep the same shell open for all three command blocks.

```sh
go build -buildvcs=false -o routeinlet ./cmd/routeinlet
CONNECTOR_BIN=/absolute/path/to/cli-proxy-api
PILOT_DIR="$HOME/routeinlet-chatgpt-pilot"
export ROUTEINLET_CONNECTOR=http://127.0.0.1:8318
export ROUTEINLET_CONNECTOR_KEY="$(openssl rand -hex 32)"
export ROUTEINLET_KEY="$(openssl rand -hex 32)"
./routeinlet init-chatgpt-pilot "$PILOT_DIR"
```

On first setup, the command creates a new profile with an empty auth directory, a loopback listener, a single local access key, and zero configured routing and stream-bootstrap retries. It refuses to overwrite an existing profile. Keep its config and auth directory private. To resume a run, reuse the existing connector key and config.

## Connect the owner account

Start CLIProxyAPI in ordinary local file mode, with external credential stores and managed configurations disabled. Sign in with the Plus account you want to test. Keep one Codex credential in this profile. Check the account's plan, usage, credit balance, and automatic credit purchases before inference. If additional credits can be consumed, an inference result alone cannot establish included funding.

```sh
"$CONNECTOR_BIN" -config "$PILOT_DIR/config.yaml" -codex-login
"$CONNECTOR_BIN" -config "$PILOT_DIR/config.yaml" -local-model &
CONNECTOR_PID=$!
```

Once the connector reports that it is listening, retrieve its candidate model IDs. Set the exact ID you want to test and start RouteInlet. The connector can list models from a catalog. Successful inference is a separate check.

```sh
curl -fsS -H "Authorization: Bearer $ROUTEINLET_CONNECTOR_KEY" "$ROUTEINLET_CONNECTOR/v1/models"
export ROUTEINLET_MODEL='<replace with exact candidate ID>'
./routeinlet start &
GATEWAY_PID=$!
```

Once the gateway is listening, run `python3 examples/probe.py`. Save the three request IDs and the reported model from each turn. When finished, stop the two processes with `kill "$GATEWAY_PID" "$CONNECTOR_PID"`.

## Record the result

| Check | Evidence to record |
| --- | --- |
| Plan and route | Exact plan and workspace, connector commit, credential count, selected model and connector-reported model. Record a model mismatch explicitly. |
| Application | Exact-format first turn, second-turn reference recall, normal finish and complete stream terminator on each request. |
| Incremental delivery | Long-output character count, text event count, and first/middle/last arrival offsets. Preserve an inconclusive timing result. |
| Attempts and transformations | Compare gateway request IDs with connector observations where available. Record upstream attempt counts and any observed changes to message roles, content, or model. Mark unobservable details unknown. |
| Funding | Compare account-side allowance and credits before and after an isolated run. Record what can be attributed to these requests. Gateway funding stays `unknown`. |
| Quota and renewal | On a naturally observed limit or credential expiry, record the provider signal, gateway error, and recovery on a fresh request. A generic 429 leaves the quota cause unknown. |

Exclude access keys, OAuth files, account identifiers, and raw authorization headers from shared results. A completed local pilot establishes only the tested account, model, workload, and environment.
