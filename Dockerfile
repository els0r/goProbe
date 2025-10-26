# Base image on Alpine / Golang
FROM golang:1.25-alpine3.22 AS build

# Download system package dependencies
RUN apk add cmake make gcc libtool git bash musl-dev zstd-dev lz4-dev

# Upload source code
WORKDIR /app

# Install Go dependencies (allow caching)
COPY go.mod .
COPY go.sum .
COPY go.work .
COPY plugins/ ./plugins/
RUN go mod download

# Upload source code
COPY . .

# Build all binaries
RUN go build -tags jsoniter -o goProbe -pgo=auto ./cmd/goProbe
RUN go build -tags jsoniter -o global-query -pgo=auto ./cmd/global-query
RUN go build -o goQuery -pgo=auto ./cmd/goQuery

###########################################################################

FROM alpine:3.22 AS sensor

# Download system package dependencies
RUN apk add libcap zstd-dev lz4-dev

# Add user
RUN set -ex \
 && adduser -G netdev -H -u 1000 -D goprobe

# Transfer binaries from build context
COPY --from=build /app/goProbe /bin/goProbe
COPY --from=build /app/goQuery /bin/goQuery

# Set ownership
RUN set -ex \
 && chmod 750 /bin/goProbe /bin/goQuery \
 && chown goprobe /bin/goProbe /bin/goQuery \
 && chgrp netdev /bin/goProbe /bin/goQuery

# Add inheritable NET_RAW capabilities to goProbe binary
RUN setcap cap_net_raw=eip /bin/goProbe

# De-escalate privileges and define entrypoint
USER goprobe
ENTRYPOINT /bin/goProbe -config "$CONFIG_PATH"

###########################################################################

FROM alpine:3.22 AS query

# Download system package dependencies
RUN apk add libcap zstd-dev lz4-dev

# Add user
RUN set -ex \
 && adduser -H -u 1000 -D goprobe

# Transfer binaries from build context
COPY --from=build /app/global-query /bin/global-query

# De-escalate privileges and define entrypoint
USER goprobe
ENTRYPOINT /bin/global-query --config "$CONFIG_PATH" server
