# Ziti Kubernetes Admission Webhook Helm Chart

This Helm chart deploys the Ziti Kubernetes Admission Webhook, which automatically injects Ziti sidecar containers into pods to provide zero-trust networking capabilities.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- cert-manager (if using automatic certificate management)

## Installation

### 1. Prepare Ziti Admin Identity

You have two options for providing the Ziti admin identity:

#### Option A: Existing Secret (Recommended for Production)

Create a secret containing your Ziti admin identity:

```bash
# Extract certificate components from enrolled identity JSON
ziti ops unwrap path/to/ziti-admin.json

# Create secret from the extracted files
# NOTE: Must be created in the same namespace where the webhook chart will be installed
kubectl create secret generic ziti-agent-identity \
  --from-file=tls.crt=path/to/ziti-admin.cert \
  --from-file=tls.key=path/to/ziti-admin.key \
  --from-file=tls.ca=path/to/ziti-admin.ca \
  --namespace=default
```

> **📝 Certificate Requirements:**
>
> - `ziti-admin.cert/.key`: Admin identity certificate/key for Ziti Controller management API authentication (extracted from enrolled identity JSON)
> - `ziti-admin.ca`: CA bundle containing root certificates that signed the Ziti Controller's TLS certificate (extracted from enrolled identity JSON)
> - **Webhook TLS Certificate**: Managed separately by cert-manager (self-signed by default) for kube-apiserver ↔ webhook communication

#### Option B: Chart-Managed Secret (Development/Testing Only)

Provide base64-encoded certificate data via Helm values:

```bash
# Extract and base64 encode your Ziti certificates
ziti ops unwrap path/to/ziti-admin.json
CERT_B64=$(base64 -w 0 < path/to/ziti-admin.cert)
KEY_B64=$(base64 -w 0 < path/to/ziti-admin.key)
CA_B64=$(base64 -w 0 < path/to/ziti-admin.ca)

# Install with embedded secrets (automatically creates secret when values are provided)
helm install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://your-controller:1280/edge/management/v1" \
  --set identity.cert="$CERT_B64" \
  --set identity.key="$KEY_B64" \
  --set identity.ca="$CA_B64"
```

> **⚠️ Security Warning**: Option B embeds sensitive credentials in Helm values, which may be stored in version control or Helm history. Use only for development/testing.

### 2. Install the Chart

```bash
# Install in the same namespace where you created the ziti-agent-identity secret
helm install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://your-controller:1280/edge/management/v1" \
  --set namespace="default"
```

> **⚠️ Important**: The webhook deployment must be installed in the same namespace as the `ziti-agent-identity` secret, as this is where the webhook server runs and provides pod mutation services to the kube-apiserver.

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
| `controller.mgmtApi` | Ziti controller management API URL (required) | `""` |
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

### Security Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `security.podSecurityContextOverride` | Override pod security context | `false` |

### Deployment Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `deployment.image.repo` | Webhook container image repository | `"docker.io/netfoundry/ziti-k8s-agent"` |
| `deployment.image.tag` | Webhook image tag | `"latest"` |
| `deployment.image.pullPolicy` | Webhook image pull policy | `"IfNotPresent"` |
| `deployment.replicas` | Number of webhook replicas | `1` |
| `deployment.resources` | Resource requests and limits | See values.yaml |

### Certificate Management

| Parameter | Description | Default |
|-----------|-------------|---------|
| `certManager.enabled` | Enable cert-manager integration | `true` |
| `certManager.certificate.duration` | Certificate duration | `"2160h"` |
| `certManager.certificate.renewBefore` | Renew before expiry | `"360h"` |

### Identity Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `identity.secretName` | Name of secret containing admin identity | `"ziti-agent-identity"` |
| `identity.certKey` | Certificate key in secret | `"tls.crt"` |
| `identity.keyKey` | Private key key in secret | `"tls.key"` |
| `identity.caKey` | CA bundle key in secret | `"tls.ca"` |
| `identity.cert` | Base64-encoded certificate (creates secret when provided) | `""` |
| `identity.key` | Base64-encoded private key (creates secret when provided) | `""` |
| `identity.ca` | Base64-encoded CA bundle (creates secret when provided) | `""` |
| `identity.trustBundle.enabled` | Enable automatic CA bundle discovery from ConfigMap | `true` |
| `identity.trustBundle.configMapName` | ConfigMap containing Ziti Controller CA bundle | `"ziti-controller-ctrl-plane-cas"` |
| `identity.trustBundle.configMapKey` | Key in ConfigMap containing CA bundle | `"ctrl-plane-cas.crt"` |

## Usage Examples

### Basic Installation

```bash
helm install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://controller.example.com:1280/edge/management/v1"
```

### Same-Cluster Ziti Controller

When deploying in the same cluster as a Ziti Controller, the webhook can automatically discover the CA bundle:

```bash
# Extract only cert and key (CA bundle discovered automatically)
ziti ops unwrap path/to/ziti-admin.json

# Create secret without CA bundle
kubectl create secret tls ziti-agent-identity \
  --cert=path/to/ziti-admin.cert \
  --key=path/to/ziti-admin.key \
  --namespace=default

# Install webhook (will use ConfigMap for CA bundle)
helm install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://ziti-controller-ctrl:1280/edge/management/v1"
```

### Custom Trust Bundle ConfigMap

```bash
helm install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://controller.example.com:1280/edge/management/v1" \
  --set identity.trustBundle.configMapName="my-ziti-controller-cas" \
  --set identity.trustBundle.configMapKey="ca-bundle.crt"
```

### Custom DNS Configuration

```bash
helm install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://controller.example.com:1280/edge/management/v1" \
  --set sidecar.dnsUpstreamEnabled=false \
  --set sidecar.searchDomains="{custom.local,example.com}"
```

### Production Configuration

```bash
helm install ziti-webhook ./charts/ziti-webhook \
  --set controller.mgmtApi="https://controller.example.com:1280/edge/management/v1" \
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

### Check webhook logs:

```bash
kubectl logs -l app=ziti-admission-webhook -n default
```

### Verify webhook configuration:

```bash
kubectl get mutatingwebhookconfiguration ziti-tunnel-sidecar -o yaml
```

### Check certificate status:

```bash
kubectl get certificate -n default
kubectl describe certificate ziti-admission-cert -n default
```
