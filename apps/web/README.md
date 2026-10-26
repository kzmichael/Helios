# web

Tenant dashboard, live map, vehicle list with route history, and alert feed.
Next.js App Router, talking to `fleet-api` through `edge-gateway` via
`lib/api-client.ts` (REST) and `lib/ws-client.ts` (live positions).

## Pages

| Path             | Shows                                                                                                               | Backed by                                                            |
| ---------------- | ------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| `/dashboard`     | Fleets with vehicle counts, linking to their vehicles                                                               | `GET /v1/fleets`                                                     |
| `/map`           | A Leaflet map of every vehicle, geofences drawn, markers moving as positions arrive                                 | `GET /v1/positions` + `WS /v1/positions/stream`, `GET /v1/geofences` |
| `/vehicles`      | Search, filter by fleet and status, sort, page through vehicles; device status and last seen                        | `GET /v1/vehicles`, `GET /v1/fleets`                                 |
| `/vehicles/{id}` | Vehicle details, its route over the last hour to 7 days on a map with distance and speed figures, its recent alerts | `GET /v1/vehicles/{id}`, `/history`, `GET /v1/alerts?vehicleId=`     |
| `/alerts`        | Open alerts, acknowledge action                                                                                     | `GET /v1/alerts`, `POST /v1/alerts/{id}/acknowledge`                 |

## How the live map works

First paint is the REST snapshot. After that every position arrives as a JSON frame on
the WebSocket and moves a marker directly — the map component is imperative on
purpose, because a large fleet can produce hundreds of frames a second and pushing
each through React state would re-render the page for every one. A frame older than the
one already shown for a vehicle is ignored. Markers are blue (moving), dark grey
(ignition off) and light grey (no report for five minutes).

When the socket drops, the client reconnects with exponential backoff and full jitter
(the same reasoning as `notifier`'s retry), and **refetches the snapshot on every
reconnect**, because frames sent while disconnected are not replayed. The access token
goes in `?token=` for this one path only — a browser WebSocket cannot send an
`Authorization` header — and `edge-gateway` removes it before forwarding.

Map tiles come from OpenStreetMap by default. For a local or private tile server set
`NEXT_PUBLIC_MAP_TILE_URL` (and `NEXT_PUBLIC_MAP_TILE_ATTRIBUTION`); the public OSM
servers are not meant for heavy or production traffic.

## API types

`lib/api-types.ts` is **generated** from `apps/_contracts/openapi.yaml` and committed
(this app's Docker context does not include `apps/_contracts`). After changing the
contract:

```bash
pnpm gen:api
```

`lib/api-client.ts` stays hand-written, on top of those types.

## No real authentication yet

Phase 0 provides a **Use local dev account** button in the navigation when
`TRACKLINE_DEV_AUTH_ENABLED=true`. It requests a short-lived token through a
same-origin Next.js route, stores it for the browser API client, and reloads the
current page. The route calls Fleet API over the Compose network; the token is
not minted directly from browser JavaScript.

Keep this flag enabled only for the local development stack. The route returns
404 when disabled. Production authentication belongs to the identity-provider
integration planned for Phase 1.

This mechanism is Phase-0-only, tied to `fleet-api`'s `dev-auth` module — see
`apps/fleet-api/README.md`. It uses the seeded Northwind owner account.

## Running it

```bash
cd apps/web
pnpm install
pnpm dev
```

It needs `edge-gateway` (and behind it `fleet-api`) reachable at `NEXT_PUBLIC_API_BASE`
(default `http://localhost:8080/v1`) and `NEXT_PUBLIC_WS_BASE` (default
`ws://localhost:8080/v1`) — both are baked in at **build** time. The gateway's
`CORS_ALLOWED_ORIGINS` must include this app's origin. `pnpm build` passes; the live
map, vehicle list and history page were checked in a browser against a running stack
(a simulated 50-vehicle tenant moving on the map).

## What's deliberately not implemented yet

- A billing/usage page — the API exists (`/v1/usage`, `/v1/invoices`, owner role), but
  the Phase 0 dev token is an operator, so there is nothing to show it with yet
- Creating or editing fleets, vehicles and geofences from the UI — the pages read; the
  API writes
- Real authentication — Phase 1
- CDN/caching concerns, RUM — Phase 3+
