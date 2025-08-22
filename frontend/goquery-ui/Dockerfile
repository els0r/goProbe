# ---- build ----

FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* yarn.lock* pnpm-lock.yaml* ./
RUN --mount=type=cache,target=/home/nonroot/.npm npm ci --no-audit --no-fund
COPY . .
RUN npm run build

# ---- runtime (Caddy) ----

FROM caddy:2-alpine AS caddy-builder

# Use Alpine as the base for better security and fewer vulnerabilities
FROM alpine:3.19

# Install ca-certificates for HTTPS support
RUN apk add --no-cache ca-certificates

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

# caddy config
COPY --chown=caddy:caddy deploy/Caddyfile /etc/caddy/Caddyfile

# Switch to non-root user
USER caddy

EXPOSE 8080

# default command
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]
