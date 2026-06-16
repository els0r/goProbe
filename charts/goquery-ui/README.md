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

```bash
helm install goquery-ui ./charts/goquery-ui \
  --namespace observability --create-namespace \
  --set backend.url=http://global-query.observability.svc.cluster.local:8145
```

Verify locally without any routing:

```bash
kubectl -n observability port-forward svc/goquery-ui 8080:80
# open http://localhost:8080
```

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
