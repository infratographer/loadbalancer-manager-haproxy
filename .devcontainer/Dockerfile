ARG GO_VERSION=1.19

FROM nats:latest as nats

FROM mcr.microsoft.com/vscode/devcontainers/go:0-${GO_VERSION}-bullseye

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive && \
    apt-get -y install --no-install-recommends bash-completion vim

RUN go install github.com/nats-io/natscli/nats@latest
