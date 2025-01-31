# build executable
FROM golang:1.23 AS build-stage
WORKDIR /app

# Version can be passed as --build-arg VERSION=$(git describe --tags --always)
ARG VERSION=development
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -x -v -ldflags="-X 'github.com/netfoundry/ziti-k8s-agent/ziti-agent/cmd/common.Version=${VERSION}'" -o build/ ./...

#
##
#

# auto-updated by Dependabot
FROM docker.io/openziti/ziti-cli:1.2.2

### Required OpenShift Labels
LABEL name="openziti/ziti-k8s-agent" \
      maintainer="developers@openziti.org" \
      vendor="NetFoundry" \
      summary="Run the OpenZiti k8s Agent" \
      description="Run the OpenZiti k8s Agent"

# set up image as root
USER root

# install artifacts as root
COPY --from=build-stage --chmod=0755 /app/build/ziti-agent /usr/local/bin/

# drop privs
USER ziggy
ENTRYPOINT [ "ziti-agent" ]
