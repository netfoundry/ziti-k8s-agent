# ziti-k8s-agent

The agent injects a sidecar that runs a Ziti tunneler as a bi-directional proxy and nameserver for Ziti services. You may enable sidecars for all pods in a namespace or specifc pods in any namespace. For each, the agent will manage the life cycle of a Ziti identity with the roles you specify in a pod annotation.

## Namespace

A namespace will not be created. The agent may be installed in any existing namespace.

## Select Pods for Sidecar Injection

Choose a method to select the pods: namespace, pod, or both. The sidecar is injected only on pod creation.

### Select by Namespace

Select all pods in namespaces labeled `tunnel.openziti.io/enabled="true"`.

```bash
kubectl label namespace {name} tunnel.openziti.io/enabled="true"
```

The agent manifest must reflect your choice to select by namespace. Setting `SIDECAR_SELECTORS="namespace"` in the script's environment before generating the manifest will configure the mutating webhook with a `namespaceSelector`.

The `kube-system` namespace is excluded based on the advice in this [Kubernetes documentation](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#avoiding-operating-on-the-kube-system-namespace).

### Select by Pod

Select pods labeled `tunnel.openziti.io/enabled="true"` in any namespace.

```bash
kubectl patch deployment/{name} -p '{"spec":{"template":{"metadata":{"labels":{"tunnel.openziti.io/enabled":"true"}}}}}'
```

The agent manifest must reflect your choice to select by pod. Setting `SIDECAR_SELECTORS="pod"` in the script's environment before generating the manifest will configure the mutating webhook with an `objectSelector`.

### Select by Namespace and Pod

Select pods labeled `tunnel.openziti.io/enabled="true"` only in namespaces labeled `tunnel.openziti.io/enabled="true"`.

The agent manifest must reflect your choice to select by pod. Setting `SIDECAR_SELECTORS="namespace,pod"` in the script's environment before generating the manifest will configure the mutating webhook with both `namespaceSelector` and `objectSelector`. Both selectors must match for a pod to be selected.

```bash
kubectl label namespace "default" tunnel.openziti.io/enabled="true"
```

## Specify Ziti roles for Pod Identities

The Ziti agent will generate a default Ziti identity role based on the app label unless you annotate it with a comma-separated list of roles. This example adds the role `acme-api-clients` to the Ziti identity shared by all replicas of the deployment. Updating the running pod's annotation will update the Ziti identity role.

```yaml
spec:
  template:
    metadata:
      annotations:
        identity.openziti.io/role-attributes: acme-api-clients
```

```bash
kubectl patch deployment/{name} -p '{"spec":{"template":{"metadata":{"annotations":{"identity.openziti.io/role-attributes":"acme-api-clients"}}}}}'
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

### Set Optional Environment Variables

These optional variables will override defaults.

```bash
export ZITI_AGENT_NAMESPACE="default"
export CLUSTER_DNS_ZONE="cluster.local"
```

You may replace the cluster's default DNS search domains for selected pods by exporting `SEARCH_DOMAINS` as a space separated list of domain name suffixes. This may be useful if the selected pods never need to resolve the names of cluster services, but do need to resolve short names in a DNS zone that you control outside of the cluster, e.g., `ziti.internal ziti.example.com`.

### Generate a Manifest

- `IDENTITY_FILE` is the path to the JSON file from the enrollment step.
- `SIDECAR_SELECTORS` is a comma-separated list of methods by which pods are selected for sidecar injection: `namespace`, `pod`, or both (see [Select Pods for Sidecar Injection](#select-pods-for-sidecar-injection) above).

```bash
IDENTITY_FILE="ziti-k8s-agent.json" SIDECAR_SELECTORS="namespace,pod" ./generate-ziti-agent-manifest.bash > ./ziti-agent.yaml
```

### Apply the Manifest

```bash
kubectl create -f ./ziti-agent.yaml
```
