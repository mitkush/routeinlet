# ChatGPT Plus connectivity pilot

Test one project owner's ChatGPT Plus account through the Codex OAuth transport in CLIProxyAPI. The application sends the same three requests as the Claude pilot. The gateway forwards the accepted Chat Completions JSON to the local connector. CLIProxyAPI handles authentication and translates the upstream response. Record the effect of that translation as part of the test.

**Qualification status:** The [24 November 2026 owner-run pilot](pilot-results/2026-11-24-chatgpt-plus.md) passed live text inference, two-turn recall, and incremental streaming with `gpt-6.1-sol`. Per-request funding attribution, quota handling, and renewal remain unverified. The commands below exercise a local owner-run application. OpenAI also documents an app-specific Sign in with ChatGPT flow with explicit plan-usage consent. This experiment uses CLIProxyAPI's Codex login. Assess hosted, shared-owner deployment separately. [Official OpenAI integration guide](https://developers.openai.com/cookbook/articles/sign-in-with-chatgpt)

## Prepare the two processes

Use RouteInlet `main` and a CLIProxyAPI executable built from commit `9a8ca1b577ef8573c5ccf108450b93ed57716cc4`. RouteInlet requires Go 1.22+; this connector revision requires **Go 1.26.10+**. Use a toolchain satisfying both (the reported Mac pilot used Go 1.27.4). The launcher and probe require Python 3.9+ and use only its standard library.

Build the connector once from the RouteInlet checkout. The subshell returns you to RouteInlet when the build finishes. The first build downloads dependencies and may compile silently for several minutes; wait for the terminal prompt and a successful exit before continuing.

```sh
git clone https://github.com/router-for-me/CLIProxyAPI.git "$HOME/routeinlet-cliproxyapi"
(
  cd "$HOME/routeinlet-cliproxyapi" &&
  git checkout --detach 9a8ca1b577ef8573c5ccf108450b93ed57716cc4 &&
  go build -o cli-proxy-api ./cmd/server
)
export CONNECTOR_BIN="$HOME/routeinlet-cliproxyapi/cli-proxy-api"
test -x "$CONNECTOR_BIN"
```

If the checkout already exists, inspect it and use the checkout/build steps there rather than cloning over it. `CONNECTOR_BIN` must point to the built executable; a placeholder path will not work. The login flag and config layout are revision-specific. [CLI entry point](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/cmd/server/main.go) · [Configuration reference](https://github.com/router-for-me/CLIProxyAPI/blob/9a8ca1b577ef8573c5ccf108450b93ed57716cc4/config.example.yaml)

Use a private local directory outside the checkout. The following shell example assumes macOS or Linux and a free gateway port 8787 and connector port 8318. Run the commands from the RouteInlet checkout in the same shell. Keep both access keys in that shell for the later start and probe commands.

```sh
go build -buildvcs=false -o routeinlet ./cmd/routeinlet
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

## Start, inspect, and stop the pilot

Select the candidate used in the recorded pilot, then start both services with one command. Login must already have succeeded. `start` returns after local readiness checks; it does not send an inference request.

```sh
export ROUTEINLET_MODEL='gpt-6.1-sol'
python3 examples/chatgpt_pilot.py start
python3 examples/chatgpt_pilot.py status
```

The launcher reports the active model, both PIDs, and the private log directory. Repeating `start` with the same settings reports the existing instance. Changing the model, either access key, or executable path requires an explicit stop and start; exporting a variable cannot reconfigure a running gateway. A mismatched connector key is still rejected by the profile check.

Once started, inspect candidate model IDs if needed:

```sh
curl -fsS -H "Authorization: Bearer $ROUTEINLET_CONNECTOR_KEY" "$ROUTEINLET_CONNECTOR/v1/models"
```

The connector can list models from an embedded catalog. Successful inference is a separate check. To try another listed text model:

```sh
python3 examples/chatgpt_pilot.py stop
export ROUTEINLET_MODEL='<exact candidate ID from the catalog>'
python3 examples/chatgpt_pilot.py start
```

`status` and `stop` also work from a new terminal without the access keys. They use `PILOT_DIR` if set, otherwise `$HOME/routeinlet-chatgpt-pilot`; pass `--directory /absolute/path/to/profile` for a different location. To run the probe or start again from a new terminal, restore the same environment values securely. Do not reinitialize the profile or generate a new connector key to resume it.

```sh
python3 examples/chatgpt_pilot.py stop
```

The launcher holds a profile lock and supervises both children. If either child exits, it shuts down its peer. Stop requests go to a private-token-protected loopback control port; the launcher never sends signals to PIDs read from a stale file. Runtime files and logs are under `$PILOT_DIR/run` with private permissions. Logs may contain connector account details; keep them local.

### Existing manual processes

If a previous manual launch owns a port, `start` reports the occupied port and leaves that process alone. Inspect it before stopping it:

```sh
lsof -nP -iTCP:8318 -sTCP:LISTEN
lsof -nP -iTCP:8787 -sTCP:LISTEN
```

Stop only the identified previous pilot processes, then use the new launcher. `status` reports managed services only; it does not adopt manual processes. A connector database lock error can also come from another process using the same auth directory. Inspect `lsof "$PILOT_DIR/auth/kv.db"`; preserve the database and credentials.

For startup failures, inspect `$PILOT_DIR/run/connector.log`, `gateway.log`, and `supervisor.log`. The launcher cleans up children it started on failure. If the supervisor is forcibly killed or the host crashes, surviving children are treated as unmanaged; inspect occupied ports before restarting. No inference is automatically retried.

## Before sending any inference request

**For an included-only run, skip live inference if paid credit consumption cannot be ruled out.** Check the selected account and workspace for spendable credits, automatic purchases, and any applicable provider-side spending controls. Turning off automatic purchases still leaves any existing balance available unless a separate control prevents its use. Record the controls or account evidence you relied on.

If the controls are unavailable, ambiguous, or allow paid usage, stop before running the probe or sending a manual completion. Record `live inference skipped — paid credit exposure unresolved`. Proceed with that exposure only if the owner separately authorizes the expenditure for this run. Record that authorization and the run as potentially paid. A gateway funding value of `unknown` does not authorize spending.

After satisfying the condition above, run `python3 examples/probe.py`. Save the three request IDs and the reported model from each turn. On completion or a skipped test, stop both processes with `python3 examples/chatgpt_pilot.py stop`.

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
