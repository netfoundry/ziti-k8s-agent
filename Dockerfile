FROM golang:1.22 as builder
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./ziti-k8s-agent ./src/./...

FROM scratch as final
COPY --chown=1001:1001 --from=builder /app/ziti-k8s-agent /ziti-k8s-agent
USER app
ENTRYPOINT ["/ziti-k8s-agent"]