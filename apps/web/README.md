# web

Tenant dashboard, live map, and alert feed. Next.js App Router, talking to
`fleet-api` via `lib/api-client.ts` and (once it exists) a realtime channel via
`lib/ws-client.ts`.

## Pages

| Path | Shows | Backed by |
|---|---|---|
| `/dashboard` | Fleet list | `GET /v1/fleets` |
| `/map` | Vehicle positions, REST snapshot + live updates | `GET /v1/positions` + WebSocket (not yet built, see below) |
| `/alerts` | Open alerts, acknowledge action | `GET /v1/alerts`, `POST /v1/alerts/{id}/acknowledge` |

## Honest gap: the realtime channel doesn't exist yet

`lib/ws-client.ts` connects to `/v1/positions/stream`, which is **not defined**
in `apps/_contracts/openapi.yaml` and not implemented in `fleet-api`. The map
page degrades gracefully — it shows the REST snapshot and reports "REST snapshot
only" — but the live-update path is unfinished application work. Adding it
(either a WebSocket upgrade on `fleet-api` or a small dedicated realtime
service) is the next real piece of Phase 0 app work, not a platform task.

## No real authentication yet

There is no login flow. `lib/api-client.ts` reads a bearer token from
`window.localStorage.getItem('trackline_dev_token')`. Set it manually in the
browser console for now:

```js
localStorage.setItem('trackline_dev_token', '<token from fleet-api /dev/token/...>')
```

This entire mechanism is Phase-0-only, tied to `fleet-api`'s `dev-auth` module —
see `apps/fleet-api/README.md`. A real login page arrives once an identity
provider exists (Phase 1).

## Running it

```bash
cd apps/web
pnpm install
pnpm dev
```

**Not yet run** — depends on `fleet-api` and `edge-gateway` being up and
reachable at `NEXT_PUBLIC_API_BASE` (defaults to `http://localhost:8080/v1`).

## What's deliberately not implemented yet

- The realtime positions channel (above)
- An actual map widget (MapLibre/Leaflet) — the `/map` page is a table today
- `/vehicles/{id}/history` (route history view)
- CDN/caching concerns, RUM — Phase 3+
