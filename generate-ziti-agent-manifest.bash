#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

: "${ZITI_AGENT_NAMESPACE:=default}"
: "${CLUSTER_DNS_ZONE:=cluster.local}"

checkCommand() {
    if ! command -v "$1" &>/dev/null; then
        logError "this script requires command '$1'. Please install on the search PATH and try again."
        # attempting to run the non-existent command will trigger an error exit like "command not found"
        $1
    fi
}

for BIN in sed jq; do
    checkCommand "$BIN"
done

ZITI_MGMT_API=$(jq -r .ztAPI "$IDENTITY_FILE" | sed 's/client/management/')
IDENTITY_CERT=$(jq -r .id.cert "$IDENTITY_FILE" | sed 's/pem://')
IDENTITY_KEY=$(jq -r .id.key "$IDENTITY_FILE" | sed 's/pem://')
IDENTITY_CA=$(jq -r .id.ca "$IDENTITY_FILE" | sed 's/pem://')

cat <<EOF >ziti-agent.yaml
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
  namespace: $ZITI_AGENT_NAMESPACE
spec:
  selfSigned: {}

---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ziti-admission-cert
  namespace: $ZITI_AGENT_NAMESPACE
spec:
  secretName: ziti-webhook-server-cert
  duration: 2160h # 90d
  renewBefore: 360h # 15d
  subject:
    organizations:
    - netfoundry
  commonName: ziti-admission-service.$ZITI_AGENT_NAMESPACE.svc.$CLUSTER_DNS_ZONE
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
  - ziti-admission-service.$ZITI_AGENT_NAMESPACE.svc.$CLUSTER_DNS_ZONE
  - ziti-admission-service.$ZITI_AGENT_NAMESPACE.svc.$CLUSTER_DNS_ZONE
  issuerRef:
    kind: Issuer
    name: selfsigned-issuer

---
apiVersion: v1
kind: Service
metadata:
  name: ziti-admission-service
  namespace: $ZITI_AGENT_NAMESPACE
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
  namespace: $ZITI_AGENT_NAMESPACE
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
        image: ${ZITI_AGENT_IMAGE:-docker.io/netfoundry/ziti-k8s-agent}
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
          - name: ZITI_MGMT_API
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  zitiMgmtApi
          - name: ZITI_CTRL_CERT
            valueFrom:
              secretKeyRef:
                name: ziti-ctrl-tls
                key:  tls.crt
          - name: ZITI_CTRL_KEY
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
    cert-manager.io/inject-ca-from: $ZITI_AGENT_NAMESPACE/ziti-admission-cert
webhooks:
  - name: tunnel.ziti.webhook
    admissionReviewVersions: ["v1"]
    $(
    if [[ $SIDECAR_SELECTOR == "namespace" ]]
      then
        cat <<SELECTOR
    namespaceSelector:
        matchLabels:
          openziti/ziti-tunnel: namespace
SELECTOR
    elif [[ $SIDECAR_SELECTOR == "pod" ]]
      then
        cat <<SELECTOR
      objectSelector:
        matchLabels:
          openziti/ziti-tunnel: pod
SELECTOR
    fi
    )
    rules:
      - operations: ["CREATE","UPDATE","DELETE"]
        apiGroups: [""]
        apiVersions: ["v1","v1beta1"]
        resources: ["pods"]
        scope: "*"
    clientConfig:
      service:
        name: ziti-admission-service
        namespace: $SIDECAR_SELECTOR
        port: 443
        path: "/ziti-tunnel"
      caBundle: ""
    sideEffects: None
    timeoutSeconds: 30

---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: $ZITI_AGENT_NAMESPACE
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
  namespace: $ZITI_AGENT_NAMESPACE

---
apiVersion: v1
kind: Secret
metadata:
  name: ziti-ctrl-tls
  namespace: $ZITI_AGENT_NAMESPACE
type: kubernetes.io/tls
stringData:
  tls.crt: $IDENTITY_CERT
  tls.key: $IDENTITY_KEY
  tls.ca:  $IDENTITY_CA

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ziti-ctrl-cfg
  namespace: $ZITI_AGENT_NAMESPACE
data:
  zitiMgmtApi: $ZITI_MGMT_API
  zitiRoleKey: identity.openziti.io/role-attributes
  podSecurityContextOverride: "true"
  SearchDomainList: ${SEARCH_DOMAINS:-\"\"}
EOF
