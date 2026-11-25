# SIWC ID-token verification patch

This is a downstream patch for CLIProxyAPI at
`9a8ca1b577ef8573c5ccf108450b93ed57716cc4`, the connector revision used by
RouteInlet's recorded pilot. It is preparation for a separate Sign in with
ChatGPT (SIWC) qualification run. The existing pilot, connector pin, Codex
login command, profile, and funding classification are unchanged.

The unpatched SIWC code generates a nonce but does not pass it to the token
exchange. It checks the token-response shape and direct-token scope, then
returns the credentials without verifying the ID token. This patch makes the
authorization-code exchange return credentials only after verification.
Both CLI binding and v8 management binding pass the pending attempt's nonce.

## Verification

- Use `coreos/go-oidc/v3 v3.16.0` and `go-jose/v4 v4.1.3`.
- Verify RS256 signatures using OpenAI's fixed HTTPS JWKS endpoint. Callback
  values and JWT headers cannot select another issuer or key URL. Refuse
  redirects, cache keys per auth service, and refresh on an unknown signing key.
- Require the production issuer, issued client ID as audience, an unexpired
  token, a nonempty subject, issued-at time, and the exact pending nonce.
  Reject future issued-at/not-before times beyond five seconds. Expiration
  remains strict. Validate authorized-party claims for multiple audiences and
  the access-token hash when present.
- Reject missing nonces and the dynamic-registration placeholder before code
  exchange. Keep the existing state, PKCE, and granted-scope checks.
- Return a fixed verification error, without claims, tokens, or JWKS bodies.

The synthetic tests use real RSA signatures and local/mock HTTP transports.
They cover invalid signatures, algorithms, issuer, audience, nonce, expiry,
required claims, key rotation, malformed JWKS, cancellation, and redirects.
CLI and management tests check that rejected tokens leave existing credential
files unchanged; management tests also check the live credential record.

## Prepare and validate

Requires Git and Go **1.26.10+**. From the RouteInlet checkout, choose a directory
that does not already exist:

```sh
sh scripts/prepare-siwc-connector.sh "$HOME/routeinlet-siwc-review"
(
  cd "$HOME/routeinlet-siwc-review" &&
  go vet ./internal/auth/codex ./internal/cmd ./internal/api/handlers/management &&
  go test -race ./internal/auth/codex ./internal/cmd ./internal/api/handlers/management -count=1 &&
  go test ./... -count=1 &&
  go build -o cli-proxy-api ./cmd/server
)
```

The preparation script fetches exactly the pinned commit, checks and applies the
patch, resolves the two fixed dependency versions, verifies module checksums,
and formats the applied Go files. It refuses to touch an existing directory.
Go produces the checkout's `go.mod`/`go.sum` changes; retain those with any
resulting binary for provenance. The script does not start a service, sign in,
or run inference. CI runs the affected packages with the race detector on Linux
and macOS, builds the server, and runs the full connector suite on Linux.

## Qualification boundary

This patch addresses authorization-code ID-token verification. It does not
qualify SIWC as a complete RouteInlet integration. In particular:

- Refresh rotation and journal behavior are unchanged. Refresh ID tokens are
  not newly authenticated by this patch; do not treat them as verified identity
  evidence or reuse this code as a refresh verifier.
- Account/registration association still needs review: persist the verified
  issuer/client/subject identity and enforce returning-account identity matching.
  A verified token alone does not prove it belongs to the existing outer Codex
  credential. Stable host-ID persistence before first authorization and the
  management reauthorization path also need qualification.
- No live SIWC consent, refresh, model listing, inference, or billing test was
  performed. The passing Codex-login pilot is not evidence for those paths.
- Per-request funding stays unverified. No paid fallback is added.

The patch is kept here because there is no accessible user-owned connector fork.
It can be proposed upstream after review; remove the downstream copy once an
upstream revision contains and passes the equivalent fixes. Do not silently
replace the pilot binary with this experimental build.

## Sources and attribution

Reviewed on 25 November 2026:

- [OpenAI registration and sign-in](https://developers.openai.com/siwc/token-sharing-open-source/sign-in)
- [OpenAI signature-validation example](https://developers.openai.com/siwc/website#6-verify-the-id-token)
- [OpenAI production discovery document](https://auth.openai.com/.well-known/openid-configuration)
- [Pinned connector](https://github.com/router-for-me/CLIProxyAPI/tree/9a8ca1b577ef8573c5ccf108450b93ed57716cc4)
- [OIDC verifier implementation](https://github.com/coreos/go-oidc/blob/v3.16.0/oidc/verify.go)

CLIProxyAPI's MIT license is reproduced in [LICENSE.CLIProxyAPI](LICENSE.CLIProxyAPI).
