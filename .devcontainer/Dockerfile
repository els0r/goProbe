FROM mcr.microsoft.com/devcontainers/go:1.25-bookworm

RUN rm -f /etc/apt/sources.list.d/yarn.list && apt-get update \
    && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install git \
    build-essential \
    curl

# compression libs
RUN apt install -y liblz4-dev libzstd-dev

# convenience tools
RUN apt install -y vim inetutils-ping

# for goProbe's info tests
RUN mkdir -p /var/lib/dbus/ && echo "da314207b73d8b1bdeb86e5adfa0d6cb" > /var/lib/dbus/machine-id

# go
USER vscode
