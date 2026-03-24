#!/bin/sh
set -eu

# Generate runtime env.js from environment variables, then start Caddy.
# This makes the image fully self-contained — no sidecar / initContainer needed.

mkdir -p /var/run/env

cat > /tmp/env.js.tmpl << 'TMPL'
// Generated at container startup
window.__ENV__ = {
  GQ_API_BASE_URL: "${GQ_API_BASE_URL}",
  HOST_RESOLVER_TYPES: "${HOST_RESOLVER_TYPES}",
  SSE_ON_LOAD: "${SSE_ON_LOAD}"
};
TMPL

GQ_API_BASE_URL="${GQ_API_BASE_URL:-}" \
  HOST_RESOLVER_TYPES="${HOST_RESOLVER_TYPES:-string}" \
  SSE_ON_LOAD="${SSE_ON_LOAD:-true}" \
  envsubst '$GQ_API_BASE_URL $HOST_RESOLVER_TYPES $SSE_ON_LOAD' \
  < /tmp/env.js.tmpl > /var/run/env/env.js

rm -f /tmp/env.js.tmpl

echo "env.js written with GQ_API_BASE_URL=${GQ_API_BASE_URL:-<empty/proxy>}"

# Hand off to Caddy (or whatever CMD was passed)
exec "$@"
