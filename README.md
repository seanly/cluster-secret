# cluster-secret operator

This operator can be used to propagate a single secret to all namespaces within your cluster. inspired by [registry-creds operator](https://github.com/alexellis/registry-creds).

## Usage

```yaml
apiVersion: ops.k8ops.cn/v1
kind: ClusterSecret
metadata:
    name: docker-registry-jcr
spec:
    secretRef:
      name: seed-docker-registry-jcr
      namespace: kubebuilder-demo
    namespaces:
      - default
```
