# Ziti Kubernetes Admission Webhook Helm Chart

This Helm chart deploys the Ziti Kubernetes Admission Webhook, which automatically injects Ziti sidecar containers into pods to provide zero-trust networking capabilities.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- cert-manager (if using automatic certificate management)

## Installation

### 1. Prepare Ziti Admin Identity

> **⚠️ SECURITY WARNING**: This webhook requires a Ziti admin identity with full network privileges. Handle with extreme care!

You have two options for providing the Ziti admin identity:

#### Option A: Existing Secret

Create a secret containing the admin identity JSON configuration:

```bash
# Create secret from enrolled identity JSON file
# NOTE: Must be created in the same namespace where the webhook chart will be installed
kubectl create secret generic "netfoundry-admin-identity" \
  --from-file=netfoundry-admin.json=netfoundry-admin.json \
  --namespace=netfoundry-system

# Install with existing secret
helm upgrade --install --namespace netfoundry-system "ziti-webhook" ./charts/ziti-webhook \
  --set identity.existingSecret.name="netfoundry-admin-identity"
```

> **📝 Identity Requirements:**
>
> - `netfoundry-admin.json`: Complete enrolled Ziti admin identity JSON file containing `id.ca`, `id.cert`, `id.key`, `ztAPI`, and `ztAPIs` fields
> - **Admin Privileges**: This identity must have full administrative privileges in the Ziti network
> - **Webhook TLS Certificate**: Managed separately by cert-manager (self-signed by default) for kube-apiserver ↔ webhook communication

#### Option B: Chart-Managed Secret

> **⚠️ Security Warning**: Option B embeds the complete admin identity in Helm values, which may be stored in version control or Helm history. This exposes full network administrative credentials. Use only for development/testing environments!

Provide the complete identity JSON directly via Helm values:

```bash
# Read the identity JSON and provide it directly (automatically creates secret)
# Using --set-json validates the JSON syntax immediately and prevents deployment with invalid JSON
helm upgrade --install ziti-webhook ./charts/ziti-webhook \
  --set-json identity.json="$(< netfoundry-admin.json)"
```

> **💡 Tip**: Using `--set-json` instead of `--set` or `--set-file` provides immediate JSON validation, catching syntax errors before deployment rather than during runtime.

### 2. Install the Chart

Install with existing secret (Option A):

```bash
helm upgrade --install --namespace netfoundry-system ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://your-controller:1280/edge/management/v1" \
  --set identity.existingSecret.name="netfoundry-admin-identity"
```

> **⚠️ Important**: The webhook deployment must be installed in the same namespace as your identity secret, as this is where the webhook server runs and provides pod mutation services to the kube-apiserver.

## Configuration

The following table lists the configurable parameters and their default values:

### Server Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `server.port` | Webhook server port | `9443` |
| `server.logLevel` | Log verbosity level | `2` |

### Controller Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.mgmtApi` | Ziti controller management API URL (optional - inferred from identity if not specified) | `""` |
| `controller.roleKey` | Role key for identity annotations | `"identity.openziti.io/role-attributes"` |

### Sidecar Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `sidecar.image.repo` | Sidecar container image repository | `"openziti/ziti-tunnel"` |
| `sidecar.image.tag` | Sidecar image tag | `"latest"` |
| `sidecar.image.pullPolicy` | Image pull policy | `"IfNotPresent"` |
| `sidecar.prefix` | Container name prefix | `"zt"` |
| `sidecar.identityDir` | Identity directory in sidecar | `"/ziti-tunnel"` |
| `sidecar.volumeMountName` | Volume mount name | `"ziti-identity"` |
| `sidecar.resolverIp` | DNS resolver IP (auto-discovered if empty) | `""` |
| `sidecar.dnsUpstreamEnabled` | Enable DNS upstream forwarding | `true` |
| `sidecar.dnsUnanswerable` | DNS unanswerable query disposition | `"refused"` |
| `sidecar.searchDomains` | Custom DNS search domains | `[]` |
| `sidecar.additionalArgs` | Additional arguments for ziti-tunnel sidecar (e.g., `["--verbose"]`). If not specified, `--verbose` is automatically added when webhook log level is 4 or higher. | `[]` |

### Security Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `security.podSecurityContextOverride` | Override pod security context | `false` |

### Deployment Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `deployment.name` | Deployment name | `"ziti-admission-wh-deployment"` |
| `deployment.image.repo` | Webhook container image repository | `"docker.io/netfoundry/ziti-k8s-agent"` |
| `deployment.image.tag` | Webhook image tag | `"latest"` |
| `deployment.image.pullPolicy` | Webhook image pull policy | `"IfNotPresent"` |
| `deployment.replicas` | Number of webhook replicas | `1` |
| `deployment.resources.requests.cpu` | CPU request | `"100m"` |
| `deployment.resources.requests.memory` | Memory request | `"128Mi"` |
| `deployment.resources.limits.cpu` | CPU limit | `"500m"` |
| `deployment.resources.limits.memory` | Memory limit | `"512Mi"` |

### Certificate Management

| Parameter | Description | Default |
|-----------|-------------|---------|
| `certManager.enabled` | Enable cert-manager integration | `true` |
| `certManager.issuer.name` | Certificate issuer name | `"ziti-k8s-agent-selfsigned-ca-issuer"` |
| `certManager.certificate.name` | Certificate name | `"ziti-admission-cert"` |
| `certManager.certificate.secretName` | Certificate secret name | `"ziti-webhook-server-cert"` |
| `certManager.certificate.duration` | Certificate duration | `"2160h"` |
| `certManager.certificate.renewBefore` | Renew before expiry | `"360h"` |

### Service Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.name` | Service name | `"ziti-admission-service"` |
| `service.port` | Service port | `443` |
| `service.targetPort` | Target port | `9443` |

### Webhook Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `webhook.name` | Webhook name | `"ziti-tunnel-sidecar"` |
| `webhook.failurePolicy` | Webhook failure policy | `"Fail"` |
| `webhook.selectors.enabled` | Webhook selector mode (namespace, pod, or both) | `"namespace"` |

### Identity Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `identity.existingSecret.name` | Name of existing secret containing identity JSON | `""` |
| `identity.existingSecret.key` | Key in existing secret containing identity JSON | `"netfoundry-admin.json"` |
| `identity.json` | Complete Ziti identity JSON (FOR DEVELOPMENT/TESTING ONLY) | `""` |

### Cluster DNS Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `clusterDns.zone` | Cluster DNS zone for search domains | `"cluster.local"` |

### Basic Installation

```bash
# Option 1: Create secret with custom name
kubectl create secret generic netfoundry-admin-identity \
  --from-file=netfoundry-admin.json=netfoundry-admin.json \
  --namespace=netfoundry-system

helm upgrade --install --namespace netfoundry-system ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://controller.example.com:1280/edge/management/v1" \
  --set identity.existingSecret.name="netfoundry-admin-identity"
```

```bash
# Option 2: Create secret with default name (release-name + "-identity")
kubectl create secret generic ziti-webhook-identity \
  --from-file=netfoundry-admin.json=netfoundry-admin.json \
  --namespace=netfoundry-system

helm upgrade --install --namespace netfoundry-system ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://controller.example.com:1280/edge/management/v1"
  # identity.existingSecret.name defaults to "ziti-webhook-identity" (release name + "-identity")
```

### Same-Cluster Ziti Controller

When deploying in the same cluster as a Ziti Controller:

```bash
# Create secret from enrolled identity JSON
kubectl create secret generic netfoundry-admin-identity \
  --from-file=netfoundry-admin.json=netfoundry-admin.json \
  --namespace=netfoundry-system

# Install webhook
helm upgrade --install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://ziti-controller-ctrl:1280/edge/management/v1" \
  --set identity.existingSecret.name="netfoundry-admin-identity"
```

### Development/Testing with Embedded Identity

```bash
# WARNING: Only for development/testing - embeds admin credentials in Helm values
# Using --set-json validates the JSON syntax immediately
helm upgrade --install ziti-webhook ./charts/ziti-webhook \
  --set-json identity.json="$(< netfoundry-admin.json)"
```

### Custom DNS Configuration

```bash
helm upgrade --install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://controller.example.com:1280/edge/management/v1" \
  --set sidecar.searchDomains="{custom.local,example.com}"
```

### Production Configuration

```bash
# Create production identity secret
kubectl create secret generic netfoundry-admin-identity \
  --from-file=netfoundry-admin.json=netfoundry-admin.json \
  --namespace=netfoundry-system

helm upgrade --install ziti-webhook ./charts/ziti-webhook \
  --namespace=netfoundry-system \
  --set controller.mgmtApi="https://controller.example.com:1280/edge/management/v1" \
  --set identity.existingSecret.name="netfoundry-admin-identity" \
  --set deployment.replicas=3 \
  --set deployment.resources.requests.cpu="200m" \
  --set deployment.resources.requests.memory="256Mi" \
  --set webhook.failurePolicy="Ignore"
```

## Enabling Ziti Injection

To enable Ziti sidecar injection for a namespace:

```bash
kubectl label namespace my-namespace tunnel.openziti.io/enabled=true
```

To enable for specific pods, add the label to the pod template:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    metadata:
      labels:
        tunnel.openziti.io/enabled: "true"
    spec:
      containers:
        - name: my-app
          image: my-app:latest
```

## Troubleshooting

### Check webhook logs

```bash
kubectl logs -l app=ziti-admission-webhook -n default
```

### Verify webhook configuration

```bash
kubectl get mutatingwebhookconfiguration ziti-tunnel-sidecar -o yaml
```

### Check certificate status

```bash
kubectl get certificate -n default
kubectl describe certificate ziti-admission-cert -n default
```
