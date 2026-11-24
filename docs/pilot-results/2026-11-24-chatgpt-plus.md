# ChatGPT Plus pilot — 24 November 2026

## Scope and evidence

The project owner ran the local pilot on an Apple Silicon Mac and supplied terminal output and account-usage screenshots. This report summarizes that evidence; it is not an independently reproduced provider run. No credentials, account identifiers, or screenshots are committed here.

| Item | Observed value |
| --- | --- |
| Gateway | Local RouteInlet on `127.0.0.1:8787`; PR #3 was merged before the run. The running binary's Git SHA was not captured. |
| Connector | CLIProxyAPI checkout `9a8ca1b577ef8573c5ccf108450b93ed57716cc4`, built locally. Runtime banner showed `dev`, `Commit: none`; binary provenance was not separately attested. |
| Build toolchain | `go1.27.4 darwin/arm64` |
| Login | Codex OAuth; one auth entry reported by the isolated connector |
| Connector listener | `127.0.0.1:8318` |
| Requested and reported model | `gpt-6.1-sol` |
| Complete probe run | Approximately 18:09:38–18:10:04 IST (UTC+05:30), 24 November 2026 |

## Application results

All three requests in the complete run received HTTP 200 from the connector and passed the probe's response checks.

| Fixture | Gateway request ID | Result |
| --- | --- | --- |
| Exact-format first turn | `6b98939226c536ba56e95a325d554393` | Passed; 1 text delta |
| Second-turn reference recall | `e8891a23fb1b11791a664e5caa4b69b4` | Passed; 7 text deltas; `Two-turn history passed.` |
| Long response | `16183b3d75c5bac494779bc7f7eebb73` | Passed; 515 text deltas, including 507 non-whitespace text events |

The long response contained 2,585 trimmed characters. Its first, middle, and last non-whitespace arrival offsets were 0.000, 7.556, and 16.129 seconds. The connector's log recorded 23.941 seconds for the whole request; this is a different measurement from the first-to-last text interval. The probe reported incremental text at the application and accepted normal stop plus a complete `[DONE]` event.

An earlier run showed two successful history turns but no supplied long-response result. It is not counted as another complete pass.

## Funding evidence and limits

Every gateway response reported `Gateway-Billing-Source: unknown`. Post-run screenshots showed an active Plus plan, 17% of the five-hour allowance remaining, 87% of the weekly allowance remaining, zero credit balance, automatic reload disabled, and no credit usage recorded at the time of capture.

These observations support included-allowance funding, but there was no recorded before/after baseline or per-request billing attribution. The analytics view showed aggregate daily usage and did not isolate the pilot. Funding therefore remains unverified at request level; the gateway's funding header remains `unknown`.

## Remaining qualification

- Quota classification and recovery have not been tested.
- Credential expiry, refresh, and re-login behavior have not been tested.
- Connector-internal attempt counts and transformations were not independently measured. One HTTP log entry per request does not prove one upstream attempt.
- This run used Codex login. SIWC consent and ID-token verification remain follow-up work in the [pilot guide](../ChatGPT_Plus_Pilot.md#sign-in-with-chatgpt-follow-up).
- The observations apply to this account, model, workload, and local setup. Hosted or shared-account deployment was not tested.

## Setup issues addressed by the lifecycle launcher

The operator initially used a placeholder connector executable path, then successfully built the pinned connector. Repeated manual starts caused port conflicts on both 8318 and 8787, plus a connector database lock timeout. A running gateway retained its old model after shell environment changes, causing a preflight `model_unavailable` error. Stopping that gateway and restarting it with the intended model resolved the mismatch.

The new launcher validates the executable path, checks occupied ports, reports the active model, rejects changed settings on duplicate starts, and stops its managed connector and gateway together. Its checks use local catalog and preflight requests; starting the pilot does not run the inference probe.
