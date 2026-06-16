# 1. goquery-ui Helm chart ships no in-cluster routing

Date: 2026-06-16

## Status

Accepted

## Context

The `charts/goquery-ui` Helm chart deploys the goProbe frontend
(`goprobe/frontend`) — a static SPA served by Caddy on port 5137. The chart
needs to make the UI reachable, and there are several ways to express that in a
chart: a Kubernetes `Ingress`, a service-mesh `Gateway` + route resources, or
nothing at all (just a `ClusterIP` Service).

Two facts constrain the choice:

- The runtime is built to sit **behind a gateway**. The Caddyfile sets
  `auto_https off` ("We're behind an Ingress; don't manage TLS here") — TLS and
  external routing are expected to terminate upstream.
- External routing is the **deployer's responsibility**, and the mechanism
  varies per environment: a Kubernetes `Ingress`, a service mesh, or a
  cloud load balancer. Bundling any one of these would pin the chart to a
  specific API (and, for a mesh, to CRD versions, a gateway name, and a
  topology the chart cannot know). Whatever the deployer uses, the chart's job
  is the same: expose the UI over HTTP in-cluster.

A chart that bundles routing is more "batteries-included" for a first install,
but it couples the chart's lifecycle to the cluster's routing API versions and
topology, which churn independently of the frontend.

## Decision

The chart ships **only** a `ClusterIP` Service with a named `http` port (so a
gateway/mesh can perform HTTP protocol detection and L7 routing) plus the
Deployment and its supporting objects. It bundles **no** `Ingress`, `Gateway`,
or route resources.

Operators wire external access out-of-band with whatever their cluster uses. The
`helm install` NOTES output prints a ready-to-adapt `Ingress` snippet and a
`kubectl port-forward` command for verification without any routing.

## Consequences

- The chart stays decoupled from any particular routing API, gateway name, or
  mesh topology; it works under a Kubernetes `Ingress`, a service mesh, a cloud
  load balancer, or bare `port-forward`.
- A bare `helm install` yields a UI reachable only in-cluster — there is one
  required manual step (routing) before it is externally reachable. This is
  surfaced in the chart README and NOTES.
- If a future deployment target standardises on a single routing topology, an
  optional, off-by-default routing template could be added without breaking
  existing installs. Reversible in that direction; the current choice does not
  lock it out.
