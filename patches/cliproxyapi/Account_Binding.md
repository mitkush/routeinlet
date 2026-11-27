# SIWC account binding and migration

This is the implementation follow-up to [RouteInlet PR #5](https://github.com/mitkush/routeinlet/pull/5).
It applies as patch 0003 after the ID-token verifier and scheduler review-table
patch, against the same CLIProxyAPI revision. It is experimental connector code,
not a change to the running Codex-login pilot or a completed SIWC rollout.

## Behavior

A successful authorization-code exchange carries its verified issuer, subject,
and optional signed `https://api.openai.com/auth.chatgpt_account_id` claim into
persistence. The persisted SIWC section contains `issuer`, `subject`, the issued
`client_id`, and `bound_account_id` for the associated outer Codex credential.
`verified_account_id`, when present, is the account claim from the new signed
SIWC token. These identity fields are not inferred from email or decoded tokens.

For an already identity-bound registration, CLI and v8 management authorization
reuse its issued client ID. A callback may omit that ID; a different ID is
rejected before code exchange. The new verified issuer and subject must match
the selected registration. A supplied signed account claim must match too.

Both paths capture the selected credential fields before authorization and
compare them again under the credential write lock before saving. A token
refresh, rebind, account replacement, or selected-identity change invalidates a
pending attempt. Restart authorization after such a conflict. Unrelated metadata
edits are preserved. Failed verification or a stale callback leaves the current
credential file and management's live credential record unchanged.

The SDK's new `BindCodexSIWCVerified` method performs the comparison inside its
existing file/mutation locks. The older `BindCodexSIWC` remains a trusted,
low-level SDK persistence API; it does not authenticate arbitrary token data.
The management OAuth route uses the verified entry point. The CLI's private
save helper receives the same account selection in its OAuth path.

## First binding and legacy profiles

A profile without verified SIWC identity is not automatically trusted. Initial
binding or migration first verifies the selected outer Codex ID token using the
fixed OpenAI issuer/JWKS, RS256 and the Codex client audience. It requires a
currently unexpired token with valid issuance time, a subject and a signed
account claim matching the selected credential's `account_id`.

The subsequent SIWC sign-in still uses a fresh state, nonce and PKCE challenge
and the verifier from PR #5. It must return the same verified subject and signed
account claim before the association can be saved. Legacy registrations retain
their issued client ID and reauthorize it; they do not silently create a new
registration.

**Compatibility limit:** the public SIWC documentation specifies validated
subject/client identity for SIWC registrations. It does not establish that the
Codex and SIWC clients always expose identical subjects or the same custom
workspace claim. This implementation therefore rejects missing/different claims
instead of assuming a mapping. Synthetic tests establish the check, not live
availability. If these claims are unavailable in practice, a supported association
mechanism or a separate SIWC credential model is needed before first binding can
be qualified. Renewing Codex login can resolve an expired outer token; it cannot
resolve an unsupported cross-client identity mapping.

For an old profile, retain the working credential and run qualification in an
isolated copy. If the error asks to renew Codex login, obtain a fresh selected
Codex login before retrying binding. Do not hand-edit issuer/subject fields,
copy identity from email, or remove the checks to make migration pass.

## Stable host identity

The connector writes `<auth-dir>/.siwc-host.state` atomically with mode `0600`,
under an inter-process file lock, before displaying/opening authorization.
Credential files are not rewritten just to save a host ID. Cancellation and
restart keep the same ID, and concurrent callers share it. Symlink aliases of
the auth directory resolve to the same host state.

An existing registration's device ID can seed a new host-state file. If it
conflicts with an established host ID, binding fails. Corrupt, non-private or
symlinked host-state files also fail; they are never silently regenerated.
Keep registrations from different legacy host IDs in separate runtime profiles
until an explicit migration is designed. Copying the whole auth directory to a
new host is not a supported way to create that host's registration.

## Tests and review checks

- Real RSA-signed tokens for first binding, legacy migration, and returning login.
- Invalid outer signatures/audiences/expiry, missing workspace evidence, and
  changed subjects/workspaces reject the association.
- CLI and management rejection preserve credential files; management tests also
  preserve live metadata.
- Returning management callbacks accept omitted/unchanged client IDs and reject
  replacements without exchanging a code.
- Stable host ID across concurrent creation, interrupted attempts and a fresh
  subprocess; private-file, malformed-state, symlink and conflict checks.
- A stale selection cannot replace a newer binding under the SDK's write lock.
- CI runs affected race tests on Linux/macOS, builds the server and runs the full
  ordinary connector suite on Linux. Existing upstream management race and
  WebSocket intermittency remain separately documented in PR #5.

## Deferred qualification

Refresh identity verification and rotation behavior, explicit account switching,
retained ID-token hints, and live consent/renewal/model-listing/streaming tests
remain follow-up work. Existing refresh/journal behavior is retained; refreshed
ID tokens are not newly verified by these patches. An identity-bound registration
alone is not evidence that a refreshed token is verified. Per-request funding
remains unverified and no paid fallback is added.

Sources reviewed on 26 November 2026:

- [OpenAI registration and sign-in](https://developers.openai.com/siwc/token-sharing-open-source/sign-in)
- [OpenAI accounts and sessions](https://developers.openai.com/siwc/token-sharing-open-source/profiles-and-sessions)
