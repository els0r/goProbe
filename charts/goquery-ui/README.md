# goquery-ui Helm chart

Deploys [goquery-ui](../../frontend/goquery-ui) — the goProbe network-flow
exploration frontend — into Kubernetes.

The image (`goprobe/frontend`) is a static SPA served by **Caddy** on port
`5137`. Caddy reverse-proxies the global-query backend so the browser only ever
makes same-origin requests. The chart is **frontend-only**: it assumes the
backend already runs as a Service you can reach in-cluster.

## Design

- **Minimal, routing-agnostic.** Ships a `ClusterIP` Service with a named
  `http` port and *no* Ingress/Gateway/VirtualService. External routing is the
  deployer's responsibility — wire it up with whatever your cluster uses
  (Kubernetes `Ingress`, a service-mesh gateway, etc.; see `helm install` NOTES
  output). The Caddyfile keeps `auto_https off`; TLS terminates upstream at the
  gateway.
- **Hardened by default.** Non-root (uid/gid 1000), all capabilities dropped,
  `readOnlyRootFilesystem`, seccomp `RuntimeDefault`. Writable paths
  (`/var/run/env`, `/tmp`, `/data`, `/config`) are backed by `emptyDir`.
- **Config rolls pods.** Runtime settings live in a ConfigMap; a
  `checksum/config` annotation restarts pods on change (env.js is generated once
  at container startup).

## Install

`backend.url` is **required** — there is no safe default. The chart fails to
render without it.

The chart is published to GHCR as an OCI artifact at
`oci://ghcr.io/els0r/charts/goquery-ui`. The package is public, so no
`helm registry login` is needed to pull.

> **Note:** GHCR's package page shows a `docker pull …` snippet — ignore it.
> This is a Helm chart, not a container image; use `helm`, and pass the
> version with `--version` rather than as a `:tag`.

```bash
helm install goquery-ui oci://ghcr.io/els0r/charts/goquery-ui \
  --version 0.1.1 \
  --namespace network-observability --create-namespace \
  --set backend.url=http://global-query.network-observability.svc.cluster.local:8145
```

Inspect before installing with `helm show chart oci://ghcr.io/els0r/charts/goquery-ui --version 0.1.1`.

To install from a local checkout instead (e.g. while developing the chart),
point at the source directory:

```bash
helm install goquery-ui ./charts/goquery-ui \
  --namespace network-observability --create-namespace \
  --set backend.url=http://global-query.network-observability.svc.cluster.local:8145
```

Verify locally without any routing:

```bash
kubectl -n network-observability port-forward svc/goquery-ui 8080:80
# open http://localhost:8080
```

### Use as a subchart dependency

Declare it in your own chart's `Chart.yaml` — the `repository` is the parent
OCI path, with the chart name and version as separate fields:

```yaml
dependencies:
  - name: goquery-ui
    version: 0.1.1
    repository: oci://ghcr.io/els0r/charts
```

then run `helm dependency update`.

## Key values

| Key | Default | Description |
|---|---|---|
| `backend.url` | `""` (**required**) | Caddy reverse-proxy target → `GQ_BACKEND_HOST`. |
| `config.apiBaseUrl` | `""` | Browser API base URL. Empty = same-origin proxy (recommended). |
| `config.hostResolverTypes` | `"string"` | Resolver types shown in the UI. |
| `config.sseOnLoad` | `"true"` | Enable SSE streaming by default. |
| `replicaCount` | `2` | Replicas (ignored when autoscaling is enabled). |
| `image.repository` / `image.tag` | `goprobe/frontend` / `""`→appVersion | Image. |
| `service.type` / `service.port` / `service.portName` | `ClusterIP` / `80` / `http` | Service. |
| `autoscaling.enabled` | `false` | HorizontalPodAutoscaler (needs metrics-server). |
| `pdb.enabled` | `false` | PodDisruptionBudget (`minAvailable: 1`). |
| `serviceAccount.automount` | `false` | SA token mount (SPA needs no API access). |
| `resources` | 50m/64Mi → 200m/128Mi | Requests / limits. |

See [values.yaml](./values.yaml) for the full set.
