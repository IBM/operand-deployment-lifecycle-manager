# Common Services Operator

## Overview

This is a common service operator for install the common service operator and instance.

## Prerequisites

- [dep][dep_tool] version v0.5.0+.
- [go][go_tool] version v1.12+.
- [docker][docker_tool] version 17.03+
- [kubectl][kubectl_tool] v1.11.3+
- [operator-sdk][operator_install]
- Access to a Kubernetes v1.11.3+ cluster

## Getting Started

### Cloning the repository

Checkout this Common Service Operator repository

```bash
# mkdir $GOPATH/src/github.ibm.com/IBMPrivateCloud
# cd $GOPATH/src/github.ibm.com/IBMPrivateCloud
# git clone git@github.ibm.com:IBMPrivateCloud/common-service-operator.git
# cd common-service-operator
```

### Building the operator

Build the Common Service operator image and push it to a public registry, such as quay.io:

```bash
# make image-build
# make image-push
```

### Installing

Run `make install` to install the operator. Check that the operator is running in the cluster, also check that the common service was deployed.

Following the expected result.

```bash
# kubectl get all -n common-service-operator
NAME                                           READY   STATUS    RESTARTS   AGE
pod/common-service-operator-786d699956-z7k4n   1/1     Running   0          21s

NAME                                      READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/common-service-operator   1/1     1            1           22s

NAME                                                 DESIRED   CURRENT   READY   AGE
replicaset.apps/common-service-operator-786d699956   1         1         1       22s
```

### Uninstalling

To uninstall all that was performed in the above step run `make uninstall`.

### Troubleshooting

Use the following command to check the operator logs.

```bash
# kubectl logs deployment.apps/common-service-operator -n common-service-operator
```

### Running Tests

Run `make test-e2e` to run the integration e2e tests with different options. For
more information see the [writing e2e tests](https://github.com/operator-framework/operator-sdk/blob/master/doc/test-framework/writing-e2e-tests.md) guide.

### Development

When the API or CRD changed, run `make code-dev` re-generate the code.

[dep_tool]: https://golang.github.io/dep/docs/installation.html
[go_tool]: https://golang.org/dl/
[kubectl_tool]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[docker_tool]: https://docs.docker.com/install/
[operator_sdk]: https://github.com/operator-framework/operator-sdk
[operator_install]: https://github.com/operator-framework/operator-sdk/blob/master/doc/user/install-operator-sdk.md
