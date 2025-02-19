<<<<<<< HEAD

# build executable
FROM golang:1.23 AS build-stage
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o build/ ./...

=======
# build executable
FROM golang:1.23 AS build-stage
WORKDIR /app

# Version can be passed as --build-arg VERSION=$(git describe --tags --always)
ARG VERSION=v0.0.0

RUN go env -w GOMODCACHE=/root/.cache/go-build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

COPY ./ziti-agent/ ./ziti-agent/
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -x -v -ldflags="-X 'github.com/netfoundry/ziti-k8s-agent/ziti-agent/cmd/common.Version=${VERSION}'" -o build/ ./...

>>>>>>> main
#
##
#

# auto-updated by Dependabot
<<<<<<< HEAD
FROM docker.io/openziti/ziti-cli:1.1.16
=======
FROM docker.io/openziti/ziti-cli:1.2.2
>>>>>>> main

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
<<<<<<< HEAD
ENTRYPOINT [ "ziti-agent" ]
=======
ENTRYPOINT [ "ziti-agent" ]
>>>>>>> main
