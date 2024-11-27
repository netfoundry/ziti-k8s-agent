# ziti-k8s-agent

The agent injects a sidecar that runs a Ziti tunneler as a bi-directional proxy and nameserver for Ziti services. You may enable sidecars for all pods in a namespace or specifc pods in any namespace. For each, the agent will manage the life cycle of a Ziti identity with the roles you specify in a pod annotation.

## Namespace

A namespace will not be created. The agent may be installed in any existing namespace.

## Select Pods for Sidecar Injection

Choose a method to select the pods.

### Select by Namespace

Select all pods in namespaces labeled `openziti/ziti-tunnel=namespace`.

```bash
export SIDECAR_SELECTOR="namespace"
kubectl label namespace {name} openziti/ziti-tunnel=namespace
```

### Select by Pod

Select pods labeled `openziti/ziti-tunnel=pod` in any namespace.

```bash
export SIDECAR_SELECTOR="pod"
kubectl patch deployment/{name} -p '{"spec":{"template":{"metadata":{"labels":{"openziti/ziti-tunnel":"pod"}}}}}'
```

## Specify Ziti roles for Pod Identities

The Ziti agent will generate a default role name based on the pods' app labels unless you annotate the pods selected above with a comma-separated list of Ziti identity roles.

```bash
kubectl patch deployment/example-app -p '{"spec":{"template":{"metadata":{"annotations":{"identity.openziti.io/role-attributes":"acme-api-clients"}}}}}'
```

## Create and Authorize Ziti Services

The Ziti agent will manage the lifecycle of a Ziti identity for each pod. You must create Ziti services and authorize pod identities to use the service by creating Ziti service policies that match the identity role you annotated the pods with. The selected pods may be authorized as dialing clients or binding hosts of a Ziti service by matching a Ziti dial service policy or a Ziti bind service policy.

Pods authorized to dial a Ziti service require that service to have a client intercept address config, e.g., `acme-api.ziti.internal:443`. That's the address the pod's main application will use to dial the Ziti service via the tunneler.

Pods authorized to bind a Ziti service require that service to have a host address config, e.g., `127.0.0.1:443`, representing another container's listener in the same pod. That's the address where the tunneler will forward traffic arriving via the hosted Ziti service.

## Deploy the Ziti Agent

### Prerequisities

1. an OpenZiti network - either NetFoundry Cloud or self-hosted
1. A JSON identity configuration file for an OpenZiti identity with the admin privilege
1. A K8S namespace in which to deploy the agent

### Set Environment Variables

These variable must be set.

```bash
export NF_ADMIN_IDENTITY_PATH="ziti-k8s-agent.json"
```

These optional variables will override defaults.

```bash
export ZITI_AGENT_NAMESPACE="default"
export CLUSTER_DNS_ZONE="cluster.local"
```

### Generate a Manifest

Run the provided script with the above variables exported to generate a K8S manifest.

```bash
./generate-ziti-agent-manifest.bash
```

### Apply the Manifest

```bash
kubectl create -f ziti-agent.yaml
```
