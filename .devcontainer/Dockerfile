ARG GO_VERSION=1.19

FROM nats:latest as nats

FROM mcr.microsoft.com/vscode/devcontainers/go:0-${GO_VERSION}-bullseye

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive && \
    apt-get -y install --no-install-recommends bash-completion 

ARG NATS_CLI_VERSION=0.0.35
ARG NATS_SERVER_VERSION=2.9.10
RUN go install github.com/nats-io/natscli/nats@v${NATS_CLI_VERSION} && \
    go install github.com/nats-io/nats-server/v2@v${NATS_SERVER_VERSION} && \
    go install github.com/nats-io/nkeys/nk@latest && \
    chmod -R a+w /go/pkg

COPY setup-scripts/setup-nats /usr/local/bin/
RUN chmod +x /usr/local/bin/setup-nats