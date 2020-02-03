<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Common Services Operator](#common-services-operator)
    - [Overview](#overview)
    - [Prerequisites](#prerequisites)
    - [Getting Started](#getting-started)
        - [Cloning the repository](#cloning-the-repository)
        - [Building the operator](#building-the-operator)
        - [Installing](#installing)
        - [Uninstalling](#uninstalling)
        - [Troubleshooting](#troubleshooting)
        - [Running Tests](#running-tests)
        - [Development](#development)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

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
# mkdir $GOPATH/src/github.com/IBM
# cd $GOPATH/src/github.com/IBM
# git clone git@github.com:IBM/common-service-operator.git
# cd common-service-operator
```

### Building the operator

Build the Common Service operator image and push it to a public registry, such as quay.io:

```bash
# make image
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

[End to end tests](./docs/e2e.md)
For more information see the [writing e2e tests](https://github.com/operator-framework/operator-sdk/blob/master/doc/test-framework/writing-e2e-tests.md) guide.

### Development

When the API or CRD changed, run `make code-dev` re-generate the code.

[dep_tool]: https://golang.github.io/dep/docs/installation.html
[go_tool]: https://golang.org/dl/
[kubectl_tool]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[docker_tool]: https://docs.docker.com/install/
[operator_sdk]: https://github.com/operator-framework/operator-sdk
[operator_install]: https://github.com/operator-framework/operator-sdk/blob/master/doc/user/install-operator-sdk.md
