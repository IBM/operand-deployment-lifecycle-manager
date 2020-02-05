# Install the common service operator on vanila Kubernetes

In this document, we will show you how to deploy and use the meta operator on the vanila Kubernetes.

## Deploy meta operator

### 1. Deploy a Kubernetes cluster

In this document, we will deploy a Kubernetes cluster by [kind](https://github.com/kubernetes-sigs/kind), which is a tool for running local Kubernetes clusters using Docker container.

If you have go (1.11+) and docker installed, you can run the following commnad to generate a Kubernetes cluster.

```bash
GO111MODULE="on" go get sigs.k8s.io/kind@v0.7.0 && kind create cluster
```

For more information see the [kind](https://github.com/kubernetes-sigs/kind#installation-and-usage)

### 2. Install OLM

Dowload and install operator lifecycle manager

For example, install operator lifecycle manager at version 0.13.0.

```bash
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.13.0/install.sh | bash -s 0.13.0
```

### 3. Create CatalogSource

```yaml
kubectl apply -f - <<END
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: opencloud-operators
  namespace: olm
spec:
  sourceType: grpc
  image: quay.io/opencloudio/operator-registry:latest
END
```

### 4. Create Operator Namespace, OperatorGroup, Subscription

```yaml
kubectl apply -f - <<END
apiVersion: v1
kind: Namespace
metadata:
  name: common-service-operator

---
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: operatorgroup
  namespace: common-service-operator
spec:
  targetNamespaces:
  - common-service-operator

---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: common-service
  namespace: common-service-operator
spec:
  channel: alpha
  name: common-service
  source: opencloud-operators
  sourceNamespace: olm
END
```

### 5. Check Operator CSV

```bash
kubectl -n common-service-operator get csv
```

## How to use mate operator to install services

### 1. Update MetaOperatorConfig and MetaOperatorCatalog custom resource

Meta Operator defines three custom resource definitions `MetaOperatorConfig`, `MetaOperatorSet` and `MetaOperatorCatalog` and it creates two example custom resources for `MetaOperatorConfig` and `MetaOperatorCatalog`.

For the `MetaOperatorConfig`,
`MetaOperatorConfig` defines the individual common service CR info:

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: MetaOperatorConfig
metadata:
  name: common-service
spec:
  services:
  - name: jenkins
    spec:
      jenkins:
        service:
          port: 8081
  - name: etcd
    spec:
      etcdCluster:
        size: 1
```

Take the jenkins operator as an example.
- The `name` field defines the name of the operator.
- The `spec` field defines the `spec` configuration for each custom resource.

In this example:
The configuration for custom resource `Jenkins` is:

```yaml
      jenkins:
        service:
          port: 8081
```

will overwrite the `spec` of the custom resource `Jenkins`

```yaml
apiVersion: jenkins.io/v1alpha2
kind: Jenkins
metadata:
  ...
  name: example
  namespace: jenkins-operator
spec:
  ...
  service:
    port: 8081
    type: ClusterIP
```

For the `MetaOperatorCatalog`,
`MetaOperatorCatalog` defines the individual common service operator info:

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: MetaOperatorCatalog
metadata:
  name: common-service
spec:
  operators:
  - name: jenkins
    namespace: jenkins-operator
    channel: alpha
    packageName: jenkins-operator
    sourceName: operatorhubio-catalog
    sourceNamespace: olm
    targetNamespaces:
      - jenkins-operator
  - name: etcd
    namespace: etcd-operator
    channel: singlenamespace-alpha
    packageName: etcd
    sourceName: operatorhubio-catalog
    sourceNamespace: olm
    targetNamespaces:
      - etcd-operator
```

The `operators` list defines the operator lifecycle management information for each operator.
Taking the jenkins as an example:
- `name` is the name of the operator, which should be the same as the services name in the `MetaOperatorConfig` and `MetaOperatorSet`.
- `namespace` is the namespace the operator will be deployed in.
- `channel` is the name of a tracked channel.
- `packageName` is the name of the package in `CatalogSource` will be deployed.
- `sourceName` is the name of the `CatalogSource`.
- `sourceNamespace` is the namespace of the `CatalogSource`.
- `targetNamespaces` is a list of namespaces, which `OperaterGroup` generates RBAC access for its member Operators to get access to. `targetNamespaces` is used to control the operator dependency. `targetNamespaces` should include all the namespaces of its dependent operators and its own namespace.

### 2. Create MetaOperatorSet custom resource

`MetaOperatorSet` defines the individual common service state, such as an individual common service that should be deployed.

This is an example of the MetaOperatorSet custom resource:

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: MetaOperatorSet
metadata:
  name: common-service
spec:
  services:
  - name: jenkins
    channel: alpha
    state: present
    description: The jenkins service
  - name: etcd
    channel: singlenamespace-alpha
    state: absent
    description: The etcd service
```

- `services` is a list defines the set for each service.
- `name` is the service name, which should be the same as the services name in the `MetaOperatorConfig` and operator name in the `MetaOperatorCatalog`.
- `channel` is an optional setting, it can overwrite the `channel` defined in the `MetaOperatorCatalog`.
- `state` defines if the service should be present or absent.
- `description` is the description of the service.
