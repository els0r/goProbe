# ---- build ----

FROM node:22-alpine AS build
RUN apk add make curl
WORKDIR /app
COPY package.json Makefile ./
RUN make types
RUN --mount=type=cache,target=/home/nonroot/.npm npm install --no-audit --no-fund
COPY . .
RUN npx -y update-browserslist-db@latest && npm run build

# ---- runtime (Caddy) ----

FROM caddy:2-alpine AS caddy-builder

# Use Alpine as the base for better security and fewer vulnerabilities
FROM alpine:3.19

# Install ca-certificates for HTTPS, gettext for envsubst (env.js generation),
# and libcap to strip file capabilities from the Caddy binary (see below).
RUN apk add --no-cache ca-certificates gettext libcap

# Copy Caddy binary from the builder image
COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy

# The official Caddy binary carries cap_net_bind_service=+ep so it can bind to
# :80/:443 as non-root. BuildKit's COPY preserves that file capability. Under a
# hardened runtime (allowPrivilegeEscalation: false → no_new_privs), the kernel
# refuses to exec() a binary that carries file caps and returns EPERM
# ("caddy: Operation not permitted"). We listen on :5137 (non-privileged), so the
# capability is unnecessary — strip it to keep the binary execable when hardened.
RUN setcap -r /usr/bin/caddy

# Create a non-root user for running Caddy
RUN addgroup -g 1000 caddy && \
    adduser -D -u 1000 -G caddy -h /var/lib/caddy -s /sbin/nologin caddy
RUN mkdir -p /var/run/env /opt/app/www /etc/caddy /data /config && \
    chown -R caddy:caddy /var/run/env /opt/app/www /etc/caddy /data /config

# Point Caddy's storage at the dirs we create + back with writable volumes.
# The official caddy image sets these; our alpine base does not, so without them
# Caddy falls back to $HOME/.local/share & $HOME/.config — which are on the
# read-only rootfs in production, producing errors on startup. (The /data and
# /config volume mounts in the Helm chart / compose are inert until this is set.)
ENV XDG_DATA_HOME=/data \
    XDG_CONFIG_HOME=/config

# static files go to a read-only dir
COPY --from=build --chown=caddy:caddy /app/dist /opt/app/www
## ensure protocol map is present as a static asset as well
COPY --from=build --chown=caddy:caddy /app/src/api/proto-map.json /opt/app/www/proto-map.json

# caddy config + entrypoint
COPY --chown=caddy:caddy deploy/Caddyfile /etc/caddy/Caddyfile
COPY --chmod=755 deploy/docker-entrypoint.sh /docker-entrypoint.sh

# Switch to non-root user
USER caddy

EXPOSE 5137

# Runtime env vars with defaults (override in K8s Deployment / docker-compose).
# GQ_BACKEND_HOST defaults to the global-query address of the single-host
# docker-compose setup; the Caddyfile applies the same fallback when it is unset
# or empty, so the proxy always has an upstream to dial (see #480).
ENV GQ_API_BASE_URL="" \
    HOST_RESOLVER_TYPES="string" \
    SSE_ON_LOAD="true" \
    GQ_BACKEND_HOST="http://127.0.0.1:8146"

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]
