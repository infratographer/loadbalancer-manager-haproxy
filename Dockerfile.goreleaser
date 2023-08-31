FROM gcr.io/distroless/static

# Copy the binary that goreleaser built
COPY loadbalancer-manager-haproxy /loadbalancer-manager-haproxy

ENTRYPOINT ["/loadbalancer-manager-haproxy"]
