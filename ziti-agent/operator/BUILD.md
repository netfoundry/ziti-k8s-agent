# Build

## Build the Image for Local Development

```bash
make
```

## Install CRDs Local Development

```bash
make install
```

## Run the Image for Local Development

```bash
make run
```

## Override the Default Image

```bash
export IMG_TAG=local
export IMG_REPO=ziti-operator
```

## Build for deployment

```bash
make docker-build
make docker-push
```

## Generate the Manifests for CRDs, Roles , Service and Deployment for Operator in `dist/ziti-operator.yaml` directory

```bash
make build-installer
```
