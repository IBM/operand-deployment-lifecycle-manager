[![Docker Repository on Quay](https://quay.io/repository/opencloudio/odlm/status "Docker Repository on Quay")](https://quay.io/repository/opencloudio/odlm)
[![License](http://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)
[![Go Report Card](https://goreportcard.com/badge/github.com/IBM/operand-deployment-lifecycle-manager)](https://goreportcard.com/report/github.com/IBM/operand-deployment-lifecycle-manager)
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Operand Deployment Lifecycle Manager (ODLM)](#operand-deployment-lifecycle-manager-odlm)
  - [Overview](#overview)
  - [Supported platforms](#supported-platforms)
  - [Prerequisites](#prerequisites)
  - [Documentation](#documentation)
  - [Developer guide](#developer-guide)
    - [Cloning the repository](#cloning-the-repository)
    - [Building the operator](#building-the-operator)
    - [Installing](#installing)
    - [Uninstalling](#uninstalling)
    - [Troubleshooting](#troubleshooting)
    - [Running Tests](#running-tests)
    - [Development](#development)
  - [SecurityContextConstraints Requirements](#securitycontextconstraints-requirements)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Operand Deployment Lifecycle Manager (ODLM)

## Overview

**Note:** Documents in this repo are in active development. For the official documentation, see [IBM Knowledge Center](https://www.ibm.com/support/knowledgecenter/SSHKN6/installer/1.x.x/index.html).

Operand Deployment Lifecycle Manager is used to manage the lifecycle of a group of operands. Check the design document [here](./docs/design/operand-deployment-lifecycle-manager.md).

Operand Deployment Lifecycle Manager has four CRDs:

| Resource                 | Short Name | Description                                                                                |
|--------------------------|------------|--------------------------------------------------------------------------------------------|
| OperandRequest | opreq | It defines which operator/operand want to be installed in the cluster |
| OperandRegistry | opreg | It defines the OLM information, like channel and catalog source, for each operator|
| OperandConfig | opcon | It defines the parameters that should be used to install the operator's operand |
| OperandBindInfo | opbi | It identifies secrets and/or configmaps that should be shared with requests |

## Supported platforms

You can install the Operand Deployment Lifecycle Manager on Linux® x86_64 with Red Hat® OpenShift® Container Platform version 4.3+.

## Prerequisites

- [operator-sdk][operator_sdk] version v1.3.0.
- [go][go_tool] version 1.15.7+
- [oc][oc_tool] version v3.11+ or [kubectl][kubectl_tool] v1.11.3+
- Access to an Openshift v4.3+ cluster

## Documentation

<!-- For installation and configuration, see [IBM Knowledge Center link](https://www.ibm.com/support/knowledgecenter/en/SSHKN6/installer/landing_installer.html). -->
- [installation](./docs/install/install.md)
- [design](./docs/design/operand-deployment-lifecycle-manager.md)

## Developer guide

### Cloning the repository

Checkout this Operand Deployment Lifecycle Manager repository

```console
# git clone https://github.com/IBM/operand-deployment-lifecycle-manager.git
# cd operand-deployment-lifecycle-manager
```

### Building the operator

Build the odlm image and push it to a public registry, such as quay.io:

```console
# make build
# make images
```

### Installing

Run `make install` to install the operator. Check that the operator is running in the cluster, also check that the common service was deployed.

Following the expected result.

```console
# kubectl get all -n ibm-common-services
NAME                                           READY   STATUS    RESTARTS   AGE
pod/operand-deployment-lifecycle-manager-786d699956-z7k4n   1/1     Running   0          21s

NAME                                      READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/operand-deployment-lifecycle-manager   1/1     1            1           22s

NAME                                                 DESIRED   CURRENT   READY   AGE
replicaset.apps/operand-deployment-lifecycle-manager-786d699956   1         1         1       22s
```

### Uninstalling

To uninstall all that was performed in the above step run `make uninstall`.

### Troubleshooting

Use the following command to check the operator logs.

```console
# kubectl logs deployment.apps/operand-deployment-lifecycle-manager -n ibm-common-services
```

### Running Tests

- [End to end tests](./docs/dev/e2e.md)
For more information see the [writing e2e tests](https://github.com/operator-framework/operator-sdk/blob/master/doc/test-framework/writing-e2e-tests.md) guide.
- [scorecard](./doc/dev/scorecard.md)

### Development

When the API or CRD changed, run `make code-dev` re-generate the code.

[go_tool]: https://golang.org/dl/
[kubectl_tool]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[oc_tool]: https://docs.okd.io/3.11/cli_reference/get_started_cli.html#cli-reference-get-started-cli
[operator_sdk]: https://github.com/operator-framework/operator-sdk
[operator_install]: https://github.com/operator-framework/operator-sdk/blob/master/doc/user/install-operator-sdk.md

## SecurityContextConstraints Requirements

The Operand Deployment Lifecycle Manager supports running under the OpenShift Container Platform default restricted security context constraints.
