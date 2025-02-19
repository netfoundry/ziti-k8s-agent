
# Releaser's Guide

## How to Trigger a Release

Push a tag to GitHub like `v*.*.*` to trigger the pre-release workflow `pre-release.yml`. This can be a release semver
intended for pre-release, e.g., `v1.0.0-rc1`, or potential promotion to stable status, e.g., `v1.0.0`.

The workflow will publish a Docker image to Docker Hub tagged with the version, e.g., `ziti-k8s-agent:1.0.0` or
`ziti-k8s-agent:1.0.0-rc1` and publish a GitHub pre-release.

## How to Promote a Stable Release

A semver stable release, e.g., `1.0.0,` can be promoted to stable status in GitHub and Docker Hub. In GitHub, mark the
release as stable by un-checking "Set as a pre-release" (`isPrerelease: false`). This triggers the stable release
promotion workflow, `promote.yml`, which re-tags the Docker image with a `:latest` and major version tag, e.g., `:v1`.
Also marking the release as "latest" implies it is stable, and advertises the release in GitHub, but has no side effect
on the Docker image.
