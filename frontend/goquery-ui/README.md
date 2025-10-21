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

Run with Docker Compose from this folder.

### Dev (hot reload via webpack-dev-server)

```bash
docker compose --profile dev up
```

Open <http://localhost:5173>

### Without Docker

Start webpack in watch mode and open `index.html` in a local static server (or your browser directly):

```bash
npm run dev
```

The bundle is emitted to `dist/` and `index.html` loads it.

## Build

```bash
npm run build
```

## Deployment

### Prod-like (Caddy serves built SPA with runtime env.js)

```bash
docker compose --profile prod up --build
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
make docker-dev   # same as compose dev profile
make docker-prod  # same as compose prod profile
make docker-build # build Caddy image locally
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
