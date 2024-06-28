# ziti-k8s-agent

To deploy to your cluster for testing:

**Note: All resources in the spec are configured for namespace `ziti`. One can replace it with his/her own namespace by replacing `ziti` with a new one. `metadata: namespace: ziti`. The webhook container was precreated for the testing and it is already configured in the deployment spec `docker.io/elblag91/ziti-agent-wh:{tag}`.**

Update the secret and config map templates with the ziti controller details and some additional sidecar specific configuration in the webhook spec file.
```bash
# secret
data:
  username: "{base64|your_value}"
  password: "{base64|your_value}"

# configmap
data:
  address: "{https://your_fqdn:port}"
  zitiRoleKey: identity.openziti.io/role-attributes
  podSecurityContextOverride: "true"
  SearchDomainList: ziti,sidecar.svc
```

Run the spec
```bash
kubectl -f sidecar-injection-webhook-spec.yaml --context $CLUSTER
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

