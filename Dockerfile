FROM golang:1.22 AS build-stage
WORKDIR /app
COPY . .
RUN go build -o build/ ./...

FROM cgr.dev/chainguard/wolfi-base:latest AS build-release-stage
USER root
COPY --from=build-stage /app/build/ziti-agent /usr/local/bin/
RUN chmod 0755 /usr/local/bin/ziti-agent
USER nobody
ENTRYPOINT ["ziti-agent"]