# ziti-k8s-agent

The agent automates sidecar injection for microservices within Kubernetes. It manages identity creation and deletion on the NetFoundry Network and in Kubernetes Secrets. It deploys a mutating webhook that interacts with the Kubernetes Admission Controller using pod CRUD (Create, Read, Update, Delete) events. 

## Deployment Details

Update the secret and config map templates with the ziti controller details and some additional sidecar specific configuration in the webhook spec file.

```yaml
# secret
type: kubernetes.io/tls
stringData:
  tls.crt: $NF_ADMIN_IDENTITY_CERT
  tls.key: $NF_ADMIN_IDENTITY_KEY
  tls.ca:  $NF_ADMIN_IDENTITY_CA

# configmap
data:
  zitiMgmtApi: $NF_MGMT_API # https://{FQDN}:{PORT}/edge/management/v1                
  zitiRoleKey: identity.openziti.io/role-attributes
  podSecurityContextOverride: "false"
  SearchDomainList: "$WHITESPACE_SEPERATED_STRING" #Default cluster.local $POD_NAMESPACE.svc
```
There are two options to enable ziti tunnel proxy injection. Snippets of mutating webhook configs in [the spec](./deployment/ziti-webhook-spec.yaml) 

  1. Per Namespace

      ```shell
      namespaceSelector:
        matchLabels:
          openziti/ziti-tunnel: namespace
      ```

  1. Per Pod

      ```shell
      objectSelector:
        matchLabels:
          openziti/ziti-tunnel: pod
      ```

## Update Webhook Namespace

Replace $WEBHOOK_NAMESPACE with the new namespace you wish to dedicate to the webhook. This will not be the same namespace as the pods that will have sidecars injected, and the webook's dedicated amespace will be deleted if you uninstall the webook like `kubectl delete -f ziti-webhook-spec.yaml --context $CLUSTER`.

Run the spec.  

```bash
kubectl create -f ziti-webhook-spec.yaml --context $CLUSTER
```

Once the webhook has been deployed successfully, label the namespace or pods

1. Per namespace by adding label `openziti/ziti-tunnel=namespace`

    ```bash
    kubectl label namespace {ns name} openziti/ziti-tunnel=namespace --context $CLUSTER
    ```
    if resources are already deployed for the namespace injection, one can run this to restart all pods per deployment.

    ```bash
    kubectl rollout restart deployment/{appname} -n {ns name} --context $CLUSTER 
    ```

1. Per Pod by adding label `openziti/ziti-tunnel=pod`

    ```bash
    kubectl patch deployment/example-app -p '{"spec":{"template":{"metadata":{"labels":{"openziti/ziti-tunnel":"pod"}}}}}' -n $NAMESPACE --context $CLUSTER
    ```



**Note: The identity role attribute is set to the pod's app name if it lacks a Ziti identity role annotation. Add a Ziti identity role annotation at any time to update identity role attributes without restarting pods. If more than one replica is present in the deployment, then the deployment needs to be updated and pods will be restarted. You can avoid the rolling restart by annotating the dedployment's replicas individually.**

Environmental variable to be used for this option that will be read by the webhook.

```yaml
data:
  zitiRoleKey: identity.openziti.io/role-attributes
```

Example of key/value for the annotation. The annotation value must be a string, where roles are separated by comma if more than one needs to be configured

```bash
kubectl annotate pod/adservice-86fc68848-dgtdz identity.openziti.io/role-attributes=sales,us-east --context $CLUSTER
```

Deployment with immediate rollout restart

```bash
kubectl patch deployment/adservice -p '{"spec":{"template":{"metadata":{"annotations":{"identity.openziti.io/role-attributes":"us-east"}}}}}' --context $CLUSTER
```

**Note: You may configure a custom nameserver or the deployment will use the cluster's default resolver.**

```yaml
# This configmap option must be added
data:
  clusterDnsSvcIp: 1.1.1.1

# This env var must be added as well to the webhook deployment spec
env:
  - name: CLUSTER_DNS_SVC_IP
    valueFrom:
      configMapKeyRef:
        name: ziti-ctrl-cfg
        key:  clusterDnsSvcIp
```

## Example Deployment

**Prerequisities:**

You must have an OpenZiti self-hosted network or an NF Cloud network.

```bash
export NF_ADMIN_IDENTITY_PATH="path/to/adminUser.json created and enrolled on NF Network"
export WEBHOOK_NAMESPACE="namespace to deploy the webhook to"
export CLUSTER="cluster context name"
```

Save the following bash script in a file, audit, and execute to generate the deployment manifest.

```bash
cat > ./generate-ziti-webhook-spec.bash
: paste here and press ctrl+D to send EOF
bash ./generate-ziti-webhook-spec.bash
```

<details><summary>Webhook Spec Creation</summary><p>

```bash
#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

export CTRL_MGMT_API=$(sed "s/client/management/" <<< `jq -r .ztAPI $NF_ADMIN_IDENTITY_PATH`)
export NF_ADMIN_IDENTITY_CERT_PATH="nf_identity_cert.pem"
export NF_ADMIN_IDENTITY_KEY_PATH="nf_identity_key.pem"
export NF_ADMIN_IDENTITY_CA_PATH="nf_identity_ca.pem"
sed "s/pem://" <<< `jq -r .id.cert $NF_ADMIN_IDENTITY_PATH` > $NF_ADMIN_IDENTITY_CERT_PATH
sed "s/pem://" <<< `jq -r .id.key $NF_ADMIN_IDENTITY_PATH` > $NF_ADMIN_IDENTITY_KEY_PATH
sed "s/pem://" <<< `jq -r .id.ca $NF_ADMIN_IDENTITY_PATH` > $NF_ADMIN_IDENTITY_CA_PATH
export NF_ADMIN_IDENTITY_CERT=$(sed "s/pem://" <<< `jq .id.cert $NF_ADMIN_IDENTITY_PATH`)
export NF_ADMIN_IDENTITY_KEY=$(sed "s/pem://" <<< `jq .id.key $NF_ADMIN_IDENTITY_PATH`)
export NF_ADMIN_IDENTITY_CA=$(sed "s/pem://" <<< `jq .id.ca $NF_ADMIN_IDENTITY_PATH`)

cat <<EOF >ziti-webhook-spec.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: $WEBHOOK_NAMESPACE

---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
  namespace: $WEBHOOK_NAMESPACE
spec:
  selfSigned: {}

---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ziti-admission-cert
  namespace: $WEBHOOK_NAMESPACE
spec:
  secretName: ziti-webhook-server-cert
  duration: 2160h # 90d
  renewBefore: 360h # 15d
  subject:
    organizations:
    - netfoundry
  commonName: ziti-admission-service.$WEBHOOK_NAMESPACE.svc
  isCA: false
  privateKey:
    algorithm: RSA
    encoding: PKCS1
    size: 2048
    rotationPolicy: Always
  usages:
    - server auth
    - client auth
  dnsNames:
  - ziti-admission-service.$WEBHOOK_NAMESPACE.svc.cluster.local
  - ziti-admission-service.$WEBHOOK_NAMESPACE.svc
  issuerRef:
    kind: Issuer
    name: selfsigned-issuer

---
apiVersion: v1
kind: Service
metadata:
  name: ziti-admission-service
  namespace: $WEBHOOK_NAMESPACE
spec:
  selector:
    app: ziti-admission-webhook
  ports:
    - name: https
      protocol: TCP
      port: 443
      targetPort: 9443
  type: ClusterIP

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ziti-admission-wh-deployment
  namespace: $WEBHOOK_NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ziti-admission-webhook
  template:
    metadata:
      labels:
        app: ziti-admission-webhook
    spec:
      containers:
      - name: ziti-admission-webhook
        image: docker.io/elblag91/ziti-k8s-agent:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 9443
        args:
          - webhook
        env:
          - name: TLS-CERT
            valueFrom:
              secretKeyRef:
                name: ziti-webhook-server-cert
                key: tls.crt
          - name: TLS-PRIVATE-KEY
            valueFrom:
              secretKeyRef:
                name: ziti-webhook-server-cert
                key: tls.key
          - name: ZITI_CTRL_MGMT_API
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  zitiMgmtApi
          - name: ZITI_CTRL_ADMIN_CERT
            valueFrom:
              secretKeyRef:
                name: ziti-ctrl-tls
                key:  tls.crt
          - name: ZITI_CTRL_ADMIN_KEY
            valueFrom:
              secretKeyRef:
                name: ziti-ctrl-tls
                key:  tls.key
          - name: ZITI_ROLE_KEY
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  zitiRoleKey
          - name: POD_SECURITY_CONTEXT_OVERRIDE
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  podSecurityContextOverride
          - name: SEARCH_DOMAIN_LIST
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  SearchDomainList

---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: ziti-tunnel-sidecar
  annotations:
    cert-manager.io/inject-ca-from: $WEBHOOK_NAMESPACE/ziti-admission-cert
webhooks:
  - name: tunnel.ziti.webhook
    admissionReviewVersions: ["v1"]
    namespaceSelector:
      matchLabels:
        openziti/ziti-tunnel: enabled
    rules:
      - operations: ["CREATE","UPDATE","DELETE"]
        apiGroups: [""]
        apiVersions: ["v1","v1beta1"]
        resources: ["pods"]
        scope: "*"
    clientConfig:
      service:
        name: ziti-admission-service
        namespace: $WEBHOOK_NAMESPACE
        port: 443
        path: "/ziti-tunnel"
      caBundle: ""
    sideEffects: None
    timeoutSeconds: 30

---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: $WEBHOOK_NAMESPACE
  name: ziti-agent-wh-roles
rules:
- apiGroups: [""] # "" indicates the core API group
  resources: ["secrets"]
  verbs: ["get", "list", "create", "delete"]
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ziti-agent-wh
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ziti-agent-wh-roles
subjects:
- kind: ServiceAccount
  name: default
  namespace: $WEBHOOK_NAMESPACE

---
apiVersion: v1
kind: Secret
metadata:
  name: ziti-ctrl-tls
  namespace: $WEBHOOK_NAMESPACE
type: kubernetes.io/tls
stringData:
  tls.crt: $NF_ADMIN_IDENTITY_CERT
  tls.key: $NF_ADMIN_IDENTITY_KEY
  tls.ca:  $NF_ADMIN_IDENTITY_CA

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ziti-ctrl-cfg
  namespace: $WEBHOOK_NAMESPACE
data:
  zitiMgmtAPI: $CTRL_MGMT_API
  zitiRoleKey: identity.openziti.io/role-attributes
  podSecurityContextOverride: "true"
  SearchDomainList:
EOF
```

</p></details>

<details><summary>Deployment Spec to Cluster</summary><p>

```bash
kubectl create -f ziti-webhook-spec.yaml --context $CLUSTER
```

</p></details>
