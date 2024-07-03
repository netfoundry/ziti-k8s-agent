FROM golang:1.22 AS build-stage
WORKDIR /app
COPY src/ .
RUN go mod init ziti-k8s-agent
RUN go mod tidy
RUN go build -o build/ ./...

FROM cgr.dev/chainguard/wolfi-base:latest AS build-release-stage
USER root
COPY --from=build-stage /app/build/ziti-k8s-agent /usr/local/bin/
RUN chmod 0755 /usr/local/bin/ziti-k8s-agent
USER nobody
ENTRYPOINT ["ziti-k8s-agent"]