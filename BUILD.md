
# Build

## Build the Image for Local Development

```bash
docker build --tag ziti-k8s-agent:local --load .
```

## Override the Default Image

```bash
export ZITI_AGENT_IMAGE=ziti-k8s-agent:local
```

## Regenerate the Manifest

```bash
./generate-ziti-webhook-spec.bash
```

## Load the Image for Local Development

### KIND

```bash
kind load docker-image ziti-k8s-agent:local
```

### Minikube

```bash
minikube image load ziti-k8s-agent:local
```
