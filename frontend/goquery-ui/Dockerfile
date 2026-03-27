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

# Install ca-certificates for HTTPS and gettext for envsubst (env.js generation)
RUN apk add --no-cache ca-certificates gettext

# Copy Caddy binary from the builder image
COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy

# Create a non-root user for running Caddy
RUN addgroup -g 1001 caddy && \
    adduser -D -u 1001 -G caddy -h /var/lib/caddy -s /sbin/nologin caddy
RUN mkdir -p /var/run/env /opt/app/www /etc/caddy /data /config && \
    chown -R caddy:caddy /var/run/env /opt/app/www /etc/caddy /data /config

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

# Runtime env vars with defaults (override in K8s Deployment / docker-compose)
ENV GQ_API_BASE_URL="" \
    HOST_RESOLVER_TYPES="string" \
    SSE_ON_LOAD="true" \
    GQ_BACKEND_HOST=""

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]
