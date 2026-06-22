# Goquery UI — Network Usage Explorer

Lightweight table + graph UI for exploring network flow data via the Global Query API.

## Prerequisites

- Node.js 18+
- npm 9+

## Install

```bash
npm ci
```

## Develop

Hot reload via webpack-dev-server:

```bash
npm run dev
```

Open <http://localhost:5173>

## Build

```bash
npm run build
```

## Deployment

### Prod-like (Caddy serves built SPA with runtime env.js)

Runs the production image hardened to mirror the Helm chart's `securityContext`
(read-only rootfs, non-root, no-new-privileges, all caps dropped), so
deployment-time issues surface locally:

```bash
docker compose up --build
```

Open <http://localhost:8080>

Optional .env (next to docker-compose.yml) to override runtime values:

```bash
GQ_API_BASE_URL=http://localhost:8081
HOST_RESOLVER_TYPES=dns,cache,local
SSE_ON_LOAD=true
```

Makefile shortcuts:

```bash
make docker-up    # run the hardened image (docker compose up --build)
make docker-build # build Caddy image locally
```

### Runtime configuration

The image is configured entirely through environment variables, read at
container startup. The two networking knobs are easy to confuse:

| Variable | Consumed by | Purpose | Default |
| --- | --- | --- | --- |
| `GQ_BACKEND_HOST` | Caddy (reverse proxy) | Where Caddy forwards `/_query*` and `/-/*` **server-side**. Point it at the global-query backend. | `http://127.0.0.1:8146` |
| `GQ_API_BASE_URL` | Browser (env.js) | URL the **browser** uses to reach the API. Leave empty to route through Caddy's same-origin proxy (recommended — keeps the CSP intact and avoids CORS). | empty |
| `HOST_RESOLVER_TYPES` | Browser (env.js) | Comma-separated resolver types offered in the UI. | `string` |
| `SSE_ON_LOAD` | Browser (env.js) | Enable SSE streaming by default. | `true` |

Browser API calls go to `/_query*` on the same origin as the UI, and Caddy
proxies them on to `GQ_BACKEND_HOST` — so under normal use you only set
`GQ_BACKEND_HOST`, and leave `GQ_API_BASE_URL` empty. The default backend
(`http://127.0.0.1:8146`) matches the global-query address in the single-host
`docker-compose.yaml`; override it for other topologies, e.g. in Kubernetes:

```bash
GQ_BACKEND_HOST=http://global-query.observability.svc.cluster.local:8145
```

## Using the UI

- Time range: quick presets (5m…30d) or set exact From/To.
- Hosts Query: free text filter (sets `query_hosts`).
- Interfaces: comma-separated list (sets `ifaces`).
- Attributes: choose which columns to group by (leave blank for all).
- Condition: free text filter (ANDs expressions like `proto=TCP and (dport=80 or dport=443)`).
- Sorting / Limit: choose metric and direction; limit applies to non-time queries.
- Run: executes the query and renders results.

### Interactions

- Graph tab
  - Click an IP: opens service breakdown (proto/dport) with in/out and totals.
  - Click an interface: opens services for that host/interface (scoped by host_id + iface).
  - Click a host: shows per-interface totals for the host.
- Table tab
  - Click a row: opens a temporal drilldown (attributes = time) scoped by row’s host_id + iface and filtered by sip/dip/dport/proto.
  - Press Enter: opens temporal drilldown for the first row when no panel is open.

### Panels

- Escape closes any open panel; clicking outside also closes it.
- Unidirectional slots/cards are highlighted in red.
- Zero values are suppressed (no `0 B` / `0 pkts`).
- Temporal drilldown:
  - Header shows `HOST — IFACE` with total Bytes/Packets.
  - Only attributes visible in the table are shown in the header row.
  - Consecutive timestamps on the same day show only HH:MM:SS (timezone suffix hidden).

## Saved Views & Export

- Save view: stores current params in localStorage. Load from the dropdown.
- Export CSV: exports the current table with selected columns and totals.

## Troubleshooting

- API requests fail with `503` and Caddy logs `"no upstreams available"`: the
  reverse proxy has no backend to dial because `GQ_BACKEND_HOST` resolved to an
  empty value. Set it to your global-query address (see
  [Runtime configuration](#runtime-configuration)). Note that listing the
  variable with an empty value (e.g. `GQ_BACKEND_HOST=` or `${GQ_BACKEND_HOST:-}`)
  overrides the built-in default with empty — leave it unset to take the default.

- If the editor reports a spurious import error for a newly added view file, run a fresh typecheck:

```bash
npm run typecheck
```

- If the Global Query API changes, regenerate client types:

```bash
npm run generate:types
```

## Notes

- For development without a server, open `index.html` directly; API calls must resolve against your environment-provided endpoint if required by `client.ts`.
- This UI is a standalone package under `applications/network-observability/src/goquery-ui`.
