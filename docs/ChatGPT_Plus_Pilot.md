# ChatGPT Plus connectivity pilot

Test one project owner's ChatGPT Plus account through the Codex OAuth transport in CLIProxyAPI. The application sends the same three requests as the Claude pilot. The gateway forwards the accepted Chat Completions JSON to the local connector. CLIProxyAPI handles authentication and translates the upstream response. Record the effect of that translation as part of the test.

**Qualification status:** Live inference, included-allowance funding, quota handling, and renewal are pending. The commands below exercise a local owner-run application. OpenAI also documents an app-specific Sign in with ChatGPT flow with explicit plan-usage consent. This experiment uses CLIProxyAPI's Codex login. Assess hosted, shared-owner deployment separately. [Official OpenAI integration guide](https://developers.openai.com/cookbook/articles/sign-in-with-chatgpt)

## Prepare the two processes

Use RouteInlet `main` and a CLIProxyAPI executable built from commit `9a8ca1b577ef8573c5ccf108450b93ed57716cc4`. To build the connector from its checkout at that revision, use its Go toolchain and run `go build -o cli-proxy-api ./cmd/server`. The login flag and config layout are revision-specific. [CLI entry point](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/cmd/server/main.go) · [Configuration reference](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/config.example.yaml)

Use a private local directory outside the checkout. The following shell example assumes macOS or Linux and a free gateway port 8787 and connector port 8318. Stop the previous gateway before switching pilots. Run the commands from the RouteInlet checkout in the same shell.

```sh
go build -buildvcs=false -o routeinlet ./cmd/routeinlet
export CONNECTOR_BIN=/absolute/path/to/cli-proxy-api
export PILOT_DIR="$HOME/routeinlet-chatgpt-pilot"
export ROUTEINLET_CONNECTOR=http://127.0.0.1:8318
export ROUTEINLET_CONNECTOR_KEY="$(openssl rand -hex 32)"
export ROUTEINLET_KEY="$(openssl rand -hex 32)"
./routeinlet init-chatgpt-pilot "$PILOT_DIR"
```

On first setup, the command creates a new profile with an empty auth directory, a loopback listener, a single local access key, and zero configured routing and stream-bootstrap retries. It refuses to overwrite an existing profile. Keep its config and auth directory private. To resume a run, reuse the existing connector key and config.

## Connect the owner account

Use the launcher below for both login and serving. It changes to the pilot directory and runs `check-chatgpt-pilot` before either connector invocation. The check rejects a local `.env`, managed or external-store environment overrides (including lowercase forms), and any configuration that differs from the generated profile for this directory and key. It verifies the local config and auth directory are real filesystem entries and prints the resolved auth directory. Confirm it is the new pilot's `auth` directory before completing sign-in. On a check failure, correct the reported cause and repeat the launch.

The connector receives only `HOME` and a system `PATH`, a fixed config path, and flags selected by the launcher. The pinned connector loads `.env` from its working directory and selects Home, Postgres, object, or Git stores before falling back to the local file store. In local mode it resolves the configured absolute auth path. [Startup and store selection](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/cmd/server/main.go) · [Auth path resolution](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/internal/util/util.go)

For the first login, start with the freshly generated empty auth directory. Open the login URL printed by the connector and sign in with the intended Plus account. Keep one Codex credential in this profile. Start the server only after login succeeds.

```sh
sh examples/chatgpt-connector.sh login
```

```sh
sh examples/chatgpt-connector.sh serve &
CONNECTOR_PID=$!
```

Once the connector reports that it is listening, retrieve its candidate model IDs. Set the exact ID you want to test and start RouteInlet. The connector can list models from a catalog. Successful inference is a separate check.

```sh
curl -fsS -H "Authorization: Bearer $ROUTEINLET_CONNECTOR_KEY" "$ROUTEINLET_CONNECTOR/v1/models"
export ROUTEINLET_MODEL='<replace with exact candidate ID>'
./routeinlet start &
GATEWAY_PID=$!
```

## Before sending any inference request

**For an included-only run, skip live inference if paid credit consumption cannot be ruled out.** Check the selected account and workspace for spendable credits, automatic purchases, and any applicable provider-side spending controls. Turning off automatic purchases still leaves any existing balance available unless a separate control prevents its use. Record the controls or account evidence you relied on.

If the controls are unavailable, ambiguous, or allow paid usage, stop before running the probe or sending a manual completion. Record `live inference skipped — paid credit exposure unresolved`. Proceed with that exposure only if the owner separately authorizes the expenditure for this run. Record that authorization and the run as potentially paid. A gateway funding value of `unknown` does not authorize spending.

After satisfying the condition above, run `python3 examples/probe.py`. Save the three request IDs and the reported model from each turn. On completion or a skipped test, stop the two processes with `kill "$GATEWAY_PID" "$CONNECTOR_PID"`.

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

## Sign in with ChatGPT follow-up

The launcher currently uses `-codex-login`; it does not enable SIWC. The pinned connector includes `-siwc-bind` and starts registration with `client_id=dynamic_agent_client`. OpenAI issues the application client ID during that flow, so obtaining a separate client ID in advance is not a prerequisite for the documented local registration path. [Pinned SIWC implementation](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/internal/auth/codex/siwc_auth.go) · [Binding command](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/internal/cmd/siwc_bind.go)

Before adding SIWC to this pilot:

- Resolve identity verification in the connector. The inspected token-exchange and binding path checks for an ID token but does not verify its signature, issuer, audience, and nonce. OpenAI's [integration guide](https://developers.openai.com/cookbook/articles/sign-in-with-chatgpt) requires those checks before saving the profile. Add rejection tests for invalid tokens and mismatched claims when choosing or updating the connector revision.
- Verify that the issued client registration is retained and that the grant includes `chatgpt.tokens.use.direct`. A completed identity login alone does not establish permission to use plan allowance.
- Repeat the isolated owner-account pilot with the funding checks above. Record consent, actual inference, attempt counts, expiry/refresh behavior, and account-side allowance evidence. Keep these results pending until observed.

SIWC availability in the dependency is not evidence that this RouteInlet pilot has completed that flow or established included-allowance funding.
