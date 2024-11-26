# args used in FROM
ARG ZITI_CLI_TAG="latest"
ARG ZITI_CLI_IMAGE="docker.io/openziti/ziti-cli"

# build executable
FROM golang:1.22 AS build-stage
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o build/ ./...

#
##
#

# this builds docker.io/openziti/ziti-k8s-agent
FROM ${ZITI_CLI_IMAGE}:${ZITI_CLI_TAG}

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
