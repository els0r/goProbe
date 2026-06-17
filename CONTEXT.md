# goProbe — Context

Shared language for the project. Terms here are the canonical names; use them in
code, commits, and discussion. Decisions with lasting trade-offs live in
[docs/adr/](docs/adr/).

## goquery-ui chart releases

The `charts/goquery-ui` Helm chart is released on two distinct **avenues**.
Keep them straight — they bump different fields and fire on different events.

- **Chart-only bump (avenue 1)** — a change to the chart's templates or values.
  Bumps the chart's own `version:` and is published by `publish-chart.yml` on
  merge to `main`. Independent of the app's release tags.

- **Tagged release (avenue 2)** — an app release via a `v*.*.*` git tag. Builds
  a new `goprobe/frontend` image and, in the same pipeline, retargets the
  chart's `appVersion` to that image, patch-bumps `version:`, ships the chart,
  and commits both fields back to `main`. Prerelease tags (`-rc*`) build images
  but do not ship a chart. See ADR 0002.

- **Chart version (`version:`)** — the chart's *own* semver (`0.x` line),
  bumped on both avenues. Distinct from `appVersion`.

- **appVersion** — the `goprobe/frontend` image tag the chart is validated
  against and pins by default. `image.tag` overrides it per deployment. It is
  an exact pin, not a floating tag — installs are reproducible (ADR 0002).
