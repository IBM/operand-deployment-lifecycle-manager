<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Install the operand deployment lifecycle manager on vanila Kubernetes](#install-the-operand-deployment-lifecycle-manager-on-vanila-kubernetes)
  - [1. Deploy a Kubernetes cluster](#1-deploy-a-kubernetes-cluster)
  - [2. Install OLM](#2-install-olm)
  - [3. Create CatalogSource](#3-create-catalogsource)
  - [4. Create Operator Namespace, OperatorGroup, Subscription](#4-create-operator-namespace-operatorgroup-subscription)
  - [5. Check Operator CSV](#5-check-operator-csv)
  - [6. Create OperandRegistry and OperandConfig instance](#6-create-operandregistry-and-operandconfig-instance)
    - [Create OperandConfig](#create-operandconfig)
    - [Create OperandRegistry](#create-operandregistry)
  - [7. Create OperandRequest instance](#7-create-operandrequest-instance)
    - [Create Operand Request](#create-operand-request)
    - [Enable or Delete an Operator](#enable-or-delete-an-operator)
  - [Post-installation](#post-installation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Install the operand deployment lifecycle manager on vanila Kubernetes

In this document, we will show you how to deploy and use the operand deployment lifecycle manager on the vanila Kubernetes.

## 1. Deploy a Kubernetes cluster

In this document, we will deploy a Kubernetes cluster by [kind](https://github.com/kubernetes-sigs/kind), which is a tool for running local Kubernetes clusters using Docker container.

If you have go (1.11+) and docker installed, you can run the following commnad to generate a Kubernetes cluster.

```bash
GO111MODULE="on" go get sigs.k8s.io/kind@v0.10.0 && kind create cluster
```

For more information see the [kind](https://github.com/kubernetes-sigs/kind#installation-and-usage)

## 2. Install OLM

Dowload and install operator lifecycle manager

For example, install operator lifecycle manager at version 0.17.0.

```bash
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.17.0/install.sh | bash -s 0.17.0
```

## 3. Create CatalogSource

```yaml
kubectl apply -f - <<END
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: opencloud-operators
  namespace: olm
spec:
  displayName: IBMCS Operators
  publisher: IBM
  sourceType: grpc
  image: docker.io/ibmcom/ibm-common-service-catalog:latest
  updateStrategy:
    registryPoll:
      interval: 45m
END
```

## 4. Create Operator Namespace, OperatorGroup, Subscription

```yaml
kubectl apply -f - <<END
apiVersion: v1
kind: Namespace
metadata:
  name: odlm

---
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: operatorgroup
  namespace: odlm
spec:
  targetNamespaces:
  - odlm

---
apiVersion: v1
data:
  namespaces: odlm
kind: ConfigMap
metadata:
  name: namespace-scope
  namespace: odlm

---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: operand-deployment-lifecycle-manager
  namespace: odlm
spec:
  channel: v3.20
  name: ibm-odlm
  source: opencloud-operators
  sourceNamespace: olm
  config:
    env:
    - name: INSTALL_SCOPE
      value: namespaced
END
```

## 5. Check Operator CSV

```bash
kubectl -n ibm-common-services get csv
```

## 6. Create OperandRegistry and OperandConfig instance

### Create OperandConfig

```yaml
kubectl apply -f - <<END
apiVersion: operator.ibm.com/v1alpha1
kind: OperandConfig
metadata:
  name: common-service
  namespace: odlm
spec:
  services:
  - name: ibm-cert-manager-operator
    spec:
      certManager: {}
      issuer: {}
      certificate: {}
END
```

### Create OperandRegistry

```yaml
kubectl apply -f - <<END
apiVersion: operator.ibm.com/v1alpha1
apiVersion: operator.ibm.com/v1alpha1
kind: OperandRegistry
metadata:
  name: common-service
  namespace: odlm
spec:
  operators:
  - name: ibm-cert-manager-operator
    namespace: odlm
    channel: v3.20
    packageName: ibm-cert-manager-operator
    scope: public
    sourceName: opencloud-operators
    sourceNamespace: olm
END
```

## 7. Create OperandRequest instance

### Create Operand Request

```yaml
kubectl apply -f - <<END
apiVersion: operator.ibm.com/v1alpha1
kind: OperandRequest
metadata:
  name: common-service
  namespace: odlm
spec:
  requests:
  - registry: common-service
    registryNamespace: odlm
    operands:
    - name: ibm-cert-manager-operator
END
```

After the `OperandRequest` created, we can check if our common services install successfully by command.

```bash
kubectl get csv -A
```

### Enable or Delete an Operator

- Enable an operator, you can add the operator into the `OperandRequest`

- Delete an operator, you can remove the operator from the`OperandRequest`

## Post-installation

The operators and their custom resource would be deployed in the cluster, and thus the installation of operands will also triggered by the CR.
