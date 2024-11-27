
# Build

## Build the Image for Local Development

```bash
docker build --tag ziti-k8s-agent:local --load .
```

## Load the Image for Local Development

Load the image in your development environment.

### KIND

```bash
kind load docker-image ziti-k8s-agent:local
```

### Minikube

```bash
minikube image load ziti-k8s-agent:local
```

## Override the Default Image

```bash
export ZITI_AGENT_IMAGE=ziti-k8s-agent:local
export ZITI_AGENT_IMAGE_PULL_POLICY=Never
```

## Regenerate the Manifest

```bash
./generate-ziti-webhook-spec.bash
```
