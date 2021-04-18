# cluster-secret operator

This operator can be used to propagate a single secret to all namespaces within your cluster.

想法来源于：[registry-creds operator](https://github.com/alexellis/registry-creds)


# 使用

```yaml
apiVersion: ops.k8ops.cn/v1
kind: ClusterSecret
metadata:
    name: docker-registry-jcr
spec:
secretRef:
  name: seed-docker-registry-jcr
  namespace: kubebuilder-demo
```
使用注意clusterSecert的name不能和secretRef的name相同