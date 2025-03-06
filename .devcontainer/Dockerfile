FROM mcr.microsoft.com/devcontainers/go:1.23-bullseye

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install git \
    build-essential \
    curl

# compression libs
RUN apt install liblz4-dev libzstd-dev

# convenience tools
RUN apt install -y vim inetutils-ping

# for goProbe's info tests
RUN mkdir -p /var/lib/dbus/ && echo "da314207b73d8b1bdeb86e5adfa0d6cb" > /var/lib/dbus/machine-id

# go
USER vscode
RUN go install golang.org/x/tools/gopls@latest
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2
