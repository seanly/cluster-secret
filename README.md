# cluster-secret operator

This operator can be used to propagate a single secret to all namespaces within your cluster. inspired by [registry-creds operator](https://github.com/alexellis/registry-creds).

## Global configuration

The operator accepts a global exclude list for namespaces that should never receive propagated secrets. Entries can be exact names or shell-style globs (`*`, `?`, `[` `]`):

```yaml
# config/manager/manager.yaml
args:
  - --enable-leader-election
  - --exclude-namespaces=kube-system,kube-public,kube-node-lease,u-*,p-*
```

The `u-*` and `p-*` patterns are useful for Rancher-managed user and project namespaces.

You can also exclude individual namespaces with an annotation:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
  annotations:
    k8ops.cn/cluster-secret.ignore: "true"
```

Global excludes are checked before per-namespace annotations.

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
