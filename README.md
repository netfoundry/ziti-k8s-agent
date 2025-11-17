# ziti-k8s-agent

The Ziti Kubernetes Agent injects a sidecar that runs a Ziti tunneler as a bi-directional proxy and nameserver for Ziti services. You may enable sidecars for all pods in a namespace or specific pods in any namespace. For each, the agent will manage the life cycle of a Ziti identity with the roles you specify in a pod annotation.

## Installation

### Prerequisites

- Kubernetes cluster with cert-manager installed
- Ziti Controller with admin identity credentials
- Helm 3.x

### Install with Helm

#### Option 1: Existing Secret (PREFERRED FOR PRODUCTION)

> **⚠️ SECURITY WARNING**: This webhook requires a Ziti admin identity with full network privileges. Handle with extreme care!

Create a secret containing your complete Ziti identity JSON configuration:

```bash
# Create secret from enrolled identity JSON file
kubectl create secret generic netfoundry-admin-identity \
  --from-file=netfoundry-admin.json=netfoundry-admin.json \
  --namespace=netfoundry-system

# Install webhook from latest release
helm upgrade --install --namespace="netfoundry-system" --create-namespace ziti-webhook \
  https://github.com/netfoundry/ziti-k8s-agent/releases/latest/download/ziti-webhook-chart.tgz \
  --set identity.existingSecret.name="netfoundry-admin-identity"
```

#### Option 2: Chart-Managed Secret (FOR DEVELOPMENT/TESTING)

> **⚠️ SECURITY WARNING**: This option embeds the admin identity in Helm values, which may be stored in Helm history. Use ONLY for development/testing environments!

Provide the complete identity JSON directly via Helm values:

```bash
# Clone repository for local development
git clone https://github.com/netfoundry/ziti-k8s-agent.git
cd ziti-k8s-agent

# Install from local chart with identity JSON
# Using --set-json validates the JSON syntax immediately
helm upgrade --install --namespace="netfoundry-system" --create-namespace ziti-webhook ./charts/ziti-webhook \
  --set-json identity.json="$(<./test-admin.json)"
```

## Select Pods for Sidecar Injection

### Select by Namespace (Default)

Select all pods in namespaces labeled `tunnel.openziti.io/enabled="true"`:

```bash
# Label namespace for sidecar injection
kubectl label namespace {name} tunnel.openziti.io/enabled="true"
```

```yaml
# values.yaml
webhook:
  selectors:
    enabled: "namespace"
```

### Select by Pod

Select pods labeled `tunnel.openziti.io/enabled="true"` in any namespace:

```bash
# Label deployment for sidecar injection
kubectl patch deployment/{name} -p '{"spec":{"template":{"metadata":{"labels":{"tunnel.openziti.io/enabled":"true"}}}}}'
```

```yaml
# values.yaml
webhook:
  selectors:
    enabled: "pod"
```

### Select by Namespace and Pod

Select pods labeled `tunnel.openziti.io/enabled="true"` only in namespaces labeled `tunnel.openziti.io/enabled="true"`:

```bash
# Label both namespace and deployment
kubectl label namespace "default" tunnel.openziti.io/enabled="true"
kubectl patch deployment/{name} -p '{"spec":{"template":{"metadata":{"labels":{"tunnel.openziti.io/enabled":"true"}}}}}'
```

```yaml
# values.yaml
webhook:
  selectors:
    enabled: "namespace,pod"
```

**Note**: The `kube-system` namespace is excluded based on [Kubernetes best practices](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#avoiding-operating-on-the-kube-system-namespace).

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

1. an OpenZiti network - either NetFoundry Cloud, NetFoundry On-Premises, or self-hosted
1. A JSON identity configuration file for an OpenZiti identity with the admin privilege
1. A K8S namespace in which to deploy the agent

## Advanced Configuration

### Custom Values File

For complex deployments, create a custom values file:

```yaml
# values-production.yaml
controller:
  # Optional - if not specified, will be inferred from identity configuration
  mgmtApi: "https://ctrl0.ziti.example.com:1280/edge/management/v1"

deployment:
  image:
    repo: "docker.io/netfoundry/ziti-k8s-agent"
    tag: "v1.2.3"
    pullPolicy: "IfNotPresent"
  replicas: 2
  resources:
    requests:
      cpu: "200m"
      memory: "256Mi"
    limits:
      cpu: "1000m"
      memory: "1Gi"

sidecar:
  image:
    repo: "docker.io/openziti/ziti-tunnel"
    tag: "v0.23.0"
    pullPolicy: "IfNotPresent"
  # Custom DNS search domains for injected pods
  searchDomains:
    - "ziti.internal"
    - "ziti.example.com"
  dnsUpstreamEnabled: true
  dnsUnanswerable: "refused"

server:
  port: 9443
  logLevel: 2  # 0=errors, 1=info, 2=detailed, 3=debug, 4=trace

webhook:
  selectors:
    enabled: "namespace,pod"  # Both namespace and pod selectors
  failurePolicy: "Fail"

# Custom cluster DNS configuration
clusterDns:
  zone: "cluster.local"

# Identity configuration (use existing secret for production)
identity:
  existingSecret:
    name: "netfoundry-admin-identity"
    key: "netfoundry-admin.json"
```

### DNS Search Domains

You may replace the cluster's default DNS search domains for selected pods by configuring `sidecar.searchDomains`. This is useful when selected pods need to resolve short names in DNS zones outside the cluster:

```yaml
# values.yaml
sidecar:
  searchDomains:
    - "ziti.internal"
    - "ziti.example.com"
```

### Resource Management

Configure resource requests and limits for production deployments:

```yaml
# values.yaml
deployment:
  resources:
    requests:
      cpu: "200m"
      memory: "256Mi"
    limits:
      cpu: "1000m"
      memory: "1Gi"
```
