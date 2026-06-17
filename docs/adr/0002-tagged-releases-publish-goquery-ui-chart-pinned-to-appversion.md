# 2. Tagged releases publish the goquery-ui chart, pinned to appVersion

Date: 2026-06-17

## Status

Accepted

## Context

The `charts/goquery-ui` chart ships on two independent avenues:

- **Chart-only bump (avenue 1)** — a change to templates/values bumps the
  chart's own `version:` and is published by `publish-chart.yml` on merge to
  `main`. Independent of the app's `v*.*.*` tags.
- **Tagged release (avenue 2)** — an app release (`v*.*.*`) builds a new
  `goprobe/frontend` image; the chart's `appVersion` must be retargeted to it,
  its `version:` bumped, and a new chart shipped.

Avenue 2 was **manual**: a human hand-edited `appVersion` and `version:` in
`Chart.yaml`, merged to `main`, and let `publish-chart.yml` ship it. We want it
automated, fired by the same `v*.*.*` tag that builds the images.

Three alternatives were weighed:

1. **Default the image to `latest` (drop appVersion tracking).** Simplest CI —
   the chart never tracks `appVersion`. Rejected: it only updates with
   `pullPolicy: Always`, which lets two pods in one Deployment run different
   `latest` builds; it makes `helm install --version X` non-reproducible and
   `helm rollback` unable to revert the frontend; and it turns `appVersion` and
   the `app.kubernetes.io/version` label into a lie. That contradicts the
   chart's pinned, hardened posture (see ADR 0001). `image.tag` already exists
   for deployers who *want* to float or pin a specific build.

2. **Commit the bump to `main` and let `publish-chart.yml` re-fire.** Keeps a
   single publish path, but a push made with the default `GITHUB_TOKEN` does
   **not** trigger another workflow — so `publish-chart.yml`'s `on: push` never
   runs and the chart silently never ships. Bridging that needs a long-lived PAT
   / GitHub App token or a `repository_dispatch` hop. Rejected: a managed secret
   and broader write scope for a mechanical bump.

3. **Publish straight from the tag workflow with `helm package
   --app-version`.** `helm package` sets `appVersion` at package time, so the
   tag workflow can ship a correctly-pinned chart with no secret and no reliance
   on a re-trigger. Chosen.

The chart's `version:` is an independent semver line (`0.x`), so its next value
is state that must be carried somewhere to stay monotonic — otherwise two
consecutive tagged releases both read the same base and collide.

## Decision

A `release-chart` job in `build-docker.yml`, `needs: [build-docker]` (so the
frontend image is pushed first), gated to final tags only
(`if: !contains(github.ref_name, '-')`):

1. Checks out `main`, reads `version:` from `Chart.yaml`, **patch-bumps** it,
   and sets `appVersion` to the tag's `X.Y.Z`. Patch is the conventional Helm
   signal for an appVersion retarget; minor/major stay reserved for real
   template changes shipped via avenue 1.
2. Runs `helm package charts/goquery-ui --app-version "$VER" --version "$NEW"`,
   checks GHCR for that version (idempotent, mirroring `publish-chart.yml`), and
   `helm push`es if absent.
3. Commits both fields back to `main` with the default `GITHUB_TOKEN` as
   bookkeeping. It intentionally does **not** re-trigger `publish-chart.yml`
   (that is the very `GITHUB_TOKEN` behaviour rejected above) — and here that is
   a feature: the chart is already published, so no double publish occurs.

The commit-back is a **direct push to `main`**, not a PR, so the PR-title rules
in `pr-naming-rules.yml` never apply to it; the message uses the sanctioned
`[trivial]` prefix anyway (it is a mechanical, generated bump).

Re-runs are made idempotent by guarding on the bump's intent rather than the
chart's existence: if `appVersion` on `main` already equals the tag, the job is
a no-op. This converges the partial-failure cases (image pushed but chart not,
chart pushed but commit-back not) without ever minting a spurious second chart
version for the same release.

`image.tag` continues to override the pin for ad-hoc deployments.

## Consequences

- Installs stay reproducible and `helm rollback` stays meaningful: a chart
  version maps to one frontend image.
- No long-lived secret; the job uses the workflow's `GITHUB_TOKEN`
  (`contents: write`, `packages: write`).
- `publish-chart.yml` remains the path for avenue 1 (chart-only pushes to
  `main`). The two share package/push/idempotency steps — minor duplication that
  could later be factored into a composite action.
- Each tagged release lands one bookkeeping commit on `main`, so `main` always
  reflects the shipped chart.
- Prerelease tags (`v*.*.*-rc*`) still build images but never ship a chart.
- Edge case: two final tags pushed in quick succession serialize through the
  commit-back; a concurrency guard can be added if it ever races.
