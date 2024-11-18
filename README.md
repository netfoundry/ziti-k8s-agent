# ziti-k8s-agent

The agent automates sidecar injection for microservices within Kubernetes. It manages identity creation and deletion on the NetFoundry Network and in Kubernetes Secrets. It deploys a mutating webhook that interacts with the Kubernetes Admission Controller using pod CRUD (Create, Read, Update, Delete) events.

# deployment details

Update the secret and config map templates with the ziti controller details and some additional sidecar specific configuration in the webhook spec file.
```bash
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

# update webhook namespace
Replace $WEBHOOK_NAMESPACE with the chosen namespace.
```

Run the spec
```bash
kubectl create -f ziti-webhook-spec.yaml --context $CLUSTER
```

Once the webhook has been deployed successfully, one can enable injection per namespace by adding label `openziti/ziti-tunnel=enabled`
```bash
kubectl label namespace {ns name} openziti/ziti-tunnel=enabled --context $CLUSTER
```

if resources are already deployed in this namespace, one can run this to restart all pods per deployment.
```bash
kubectl rollout restart deployment/{appname} -n {ns name} --context $CLUSTER 
```

**Note: The Identity Role Attribute is set to the app name. One can add annotation to pods to update attributes without restarting pods. If more than one replica is present in the deployment, then the deployment needs to be updated and pods will be restarted or annotate each pod separately.**

Environmental variable to be used for this option that will be read by the webhook.
```bash
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

**Note: By default, the DNS Service ClusterIP is looked up. If one wants to configure a custom DNS server IP to overwritte the discovery, it is configurable.**

```bash
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

# Example Deployment

**Prerequisities:**

[NF Network](https://cloudziti.io/login)

```shell
export NF_IDENTITY_PATH="path/to/adminUser.json created and enrolled on NF Network"
export WEBHOOK_NAMESPACE="namespace to deploy the webhook to"
export CLUSTER="cluster context name"
```
Copy the following code to linux terminal

<details><summary>Webhook Spec Creation</summary><p>

```shell
export CTRL_MGMT_API=$(sed "s/client/management/" <<< `jq -r .ztAPI $NF_IDENTITY_PATH`)
export NF_IDENTITY_CERT_PATH="nf_identity_cert.pem"
export NF_IDENTITY_KEY_PATH="nf_identity_key.pem"
export NF_IDENTITY_CA_PATH="nf_identity_ca.pem"
sed "s/pem://" <<< `jq -r .id.cert $NF_IDENTITY_PATH` > $NF_IDENTITY_CERT_PATH
sed "s/pem://" <<< `jq -r .id.key $NF_IDENTITY_PATH` > $NF_IDENTITY_KEY_PATH
sed "s/pem://" <<< `jq -r .id.ca $NF_IDENTITY_PATH` > $NF_IDENTITY_CA_PATH
export NF_ADMIN_IDENTITY_CERT=$(sed "s/pem://" <<< `jq .id.cert $NF_IDENTITY_PATH`)
export NF_ADMIN_IDENTITY_KEY=$(sed "s/pem://" <<< `jq .id.key $NF_IDENTITY_PATH`)
export NF_ADMIN_IDENTITY_CA=$(sed "s/pem://" <<< `jq .id.ca $NF_IDENTITY_PATH`)

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

```shell
kubectl create -f ziti-webhook-spec.yaml --context $CLUSTER
```

</p></details>
