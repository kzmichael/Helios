# Authentication, Authorization & Tenant Context

Contract between the application and the platform. The application defines *what* must
be true; enforcing it at the infrastructure layer is platform work.

---

## 1. Principals

| Principal | Authenticates with | Issued by | Lifetime |
|---|---|---|---|
| **Human user** | OIDC access token (JWT, RS256) | Identity provider | 15 min access / 8 h refresh |
| **Device** | Bearer device token | Provisioning at device registration | Until revoked |
| **Machine client** | API key (`X-Trackline-Key`) | Tenant admin via API | Until revoked or expired |
| **Internal service** | Short-lived workload identity token | Platform | ≤ 1 h |

Identity provider by phase:

| Phase | Provider |
|---|---|
| 0 | Local static RS256 key pair; `fleet-api` mints dev tokens. No real IdP. |
| 1–4 | Self-hosted OIDC (Dex or Keycloak) |
| 5+ | Microsoft Entra ID, federated |

The application only ever sees a **JWT it can validate against a JWKS endpoint**. Swapping
the provider must require no application change — that is the point of using OIDC rather
than a bespoke login.

---

## 2. Access token claims

```json
{
  "iss": "https://auth.trackline.local/",
  "aud": "trackline-api",
  "sub": "22222222-0000-4000-8000-000000000002",
  "tid": "22222222-2222-4222-8222-222222222222",
  "tier": "pro",
  "roles": ["operator"],
  "scp": "fleet:read fleet:write alerts:read",
  "iat": 1789000000,
  "exp": 1789000900,
  "jti": "9f2c…"
}
```

`tid` is the **only** source of tenant identity. It is never read from a path, query
parameter, body, or inbound header.

### Validation performed by `edge-gateway`

Every one of these is mandatory. Skipping any of them is a vulnerability, not an
optimisation.

1. Signature verifies against the JWKS key matching `kid`
2. `iss` exactly matches the configured issuer
3. `aud` contains `trackline-api`
4. `exp` in the future, `nbf`/`iat` not in the future (allow ≤ 60 s clock skew)
5. `alg` matches the expected algorithm — **reject `none` and reject HMAC algorithms**
6. `tid` is present, is a UUID, and names a tenant with status `active`

JWKS is cached with a bounded TTL and refreshed on unknown `kid`. A JWKS fetch failure
must **not** cause tokens to be accepted unvalidated.

---

## 3. Tenant context propagation

`edge-gateway` is the only trust boundary. Downstream services do not re-validate the
JWT; they trust the context the gateway asserts.

```
client ──JWT──▶ edge-gateway ──X-Trackline-Tenant──▶ fleet-api
                     │                                 notifier
                     │                                 …
                validates once
```

Internal headers set by the gateway:

| Header | Content |
|---|---|
| `X-Trackline-Tenant` | Tenant UUID from `tid` |
| `X-Trackline-Subject` | `sub` |
| `X-Trackline-Roles` | Comma-separated roles |
| `X-Trackline-Request-Id` | Correlation id |
| `traceparent` | W3C trace context |

### The rule that matters most

> **`edge-gateway` MUST strip every `X-Trackline-*` header from the inbound request
> before setting its own.**

Without this, any client can send `X-Trackline-Tenant: <someone-else>` and — if the header
survives to `fleet-api` — read another tenant's data. This is a confused-deputy attack and
it is the single most likely way this system gets breached.

Two consequences for the platform:

- Internal services must be unreachable from outside the cluster. Phase 4 enforces this
  with default-deny NetworkPolicy.
- Phase 9's service mesh replaces header trust with mTLS identity. Until then, the
  network boundary *is* the control.

---

## 4. Authorization

Authorization is evaluated in `fleet-api`, not at the gateway. The gateway authenticates;
the service authorizes.

### Roles

| Role | Read | Write | Manage users | Billing |
|---|---|---|---|---|
| `owner` | ✅ | ✅ | ✅ | ✅ |
| `admin` | ✅ | ✅ | ✅ | ❌ |
| `operator` | ✅ | ✅ | ❌ | ❌ |
| `viewer` | ✅ | ❌ | ❌ | ❌ |

### Tenant isolation

Every query is scoped by `tenant_id` taken from the asserted context. A row belonging to
another tenant must be indistinguishable from a row that does not exist:

> Cross-tenant access returns **404, never 403.** A 403 confirms the resource exists,
> which is an information leak.

Phase 4 adds Postgres row-level security as a second, independent layer — because a
single missing `WHERE tenant_id = $1` should not be a data breach.

---

## 5. Device authentication

Devices present `Authorization: Bearer <device-token>` on gRPC and MQTT connections.

- Token is 32 bytes of CSPRNG output, base64url encoded
- Stored as **SHA-256 of the token** (`device.auth_token_hash`)
- Shown exactly once, at provisioning

**Why SHA-256 and not bcrypt here, when passwords use bcrypt:** bcrypt's cost exists to
slow brute force against *low-entropy* human-chosen secrets. A 256-bit random token has no
meaningful brute-force surface, and the ingest path verifies tokens at very high rate —
a deliberately slow hash would become the bottleneck. Different threat model, different
primitive. API keys follow the same reasoning.

Device tokens are verified against a cache, not the database, on the hot path. Revocation
is therefore eventually consistent, bounded by the cache TTL. That is an accepted
trade-off and should be recorded in an ADR.

---

## 6. Secrets this system needs

The application declares what it requires; the platform decides where they live.

| Secret | Consumer | Rotation |
|---|---|---|
| OIDC signing key pair | Identity provider | 90 days, overlapping |
| Database credentials | `fleet-api`, `ingest` | Phase 4: dynamic, ≤ 1 h |
| Kafka credentials | `ingest`, `notifier` | Phase 4 |
| Webhook signing secret | `notifier` | Per tenant, on demand |
| SMTP credentials | `notifier` | Provider-managed |

**No service reads a secret from an environment variable baked into an image.** How they
arrive — External Secrets, Vault injection, CSI driver — is a platform decision made in
Phase 4.

---

## 7. Webhook outbound signing

Outbound webhooks carry an HMAC so receivers can verify authenticity:

```
X-Trackline-Signature: t=1789000000,v1=<hex hmac-sha256>
X-Trackline-Delivery:  <uuid>
```

Signed payload is `"{t}.{raw_body}"`, keyed by the channel's secret. Receivers must
reject timestamps outside a ±5 minute window to prevent replay, and must compare
signatures in constant time.

---

## 8. Audit

Every state-changing operation writes to `audit_log` with actor kind, actor id, action,
target, and trace id. Authentication failures are logged **without** the presented
credential.

Never logged, at any level: tokens, API keys, device tokens, password hashes, webhook
secrets, full request bodies for auth endpoints.

---

## 9. Known gaps

Recorded honestly rather than hidden:

| Gap | Closed in |
|---|---|
| Downstream services trust gateway headers with no cryptographic proof | Phase 9 (mesh mTLS / SPIFFE) |
| Device token revocation is eventually consistent | Accepted; needs an ADR |
| No refresh-token rotation or reuse detection | Phase 4 |
| No MFA for human users | Phase 4, delegated to the IdP |
| Rate limiting is per tenant, not per principal | Phase 2 |
