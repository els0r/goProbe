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

### Listen address & exposure

Caddy's bind address is set by `GQ_LISTEN_ADDR` (host:port). The image is
**secure by default**: it binds `127.0.0.1:5137` (loopback only), so the UI is
not exposed on any external interface unless you opt in.

- **`network_mode: host`**: the default `127.0.0.1:5137` keeps the UI local to
  the host. To reach it from other machines, set `GQ_LISTEN_ADDR=:5137` (all
  interfaces) or a specific trusted interface, and front it with a reverse
  proxy / firewall. The provided host-network `docker-compose.yaml` makes the
  loopback default explicit.
- **Bridged containers / Kubernetes**: the container needs to bind all
  interfaces so published ports (`docker run -p`) and the kubelet probes /
  Service can reach it by IP — loopback would break them. These set
  `GQ_LISTEN_ADDR=:5137` explicitly: the Helm chart via `config.listenAddr`, the
  bridge `docker-compose.yml` by pinning it. Exposure is still governed by the
  published ports / Service / Ingress, not the in-container bind address.

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
