<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Install the operand deployment lifecycle manager on vanila Kubernetes](#install-the-operand-deployment-lifecycle-manager-on-vanila-kubernetes)
  - [1. Deploy a Kubernetes cluster](#1-deploy-a-kubernetes-cluster)
  - [2. Install OLM](#2-install-olm)
  - [3. Create CatalogSource](#3-create-catalogsource)
  - [4. Create Operator Namespace, OperatorGroup, Subscription](#4-create-operator-namespace-operatorgroup-subscription)
  - [5. Check Operator CSV](#5-check-operator-csv)
  - [6. Create OperandRequest instance](#6-create-operandrequest-instance)
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
GO111MODULE="on" go get sigs.k8s.io/kind@v0.7.0 && kind create cluster
```

For more information see the [kind](https://github.com/kubernetes-sigs/kind#installation-and-usage)

## 2. Install OLM

Dowload and install operator lifecycle manager

For example, install operator lifecycle manager at version 0.13.0.

```bash
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.13.0/install.sh | bash -s 0.13.0
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
  sourceType: grpc
  image: docker.io/ibmcom/ibm-common-service-catalog:latest
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
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: operand-deployment-lifecycle-manager
  namespace: odlm
spec:
  channel: beta
  name: ibm-odlm
  source: opencloud-operators
  sourceNamespace: olm
END
```

## 5. Check Operator CSV

```bash
kubectl -n ibm-common-services get csv
```

## 6. Create OperandRequest instance

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
    - name: ibm-mongodb-operator
    - name: ibm-iam-operator
    - name: ibm-monitoring-exporters-operator
    - name: ibm-monitoring-prometheusext-operator
    - name: ibm-healthcheck-operator
END
```

The list of operators you can add:

```bash
    - name: ibm-cert-manager-operator
    - name: ibm-mongodb-operator
    - name: ibm-iam-operator
    - name: ibm-monitoring-exporters-operator
    - name: ibm-monitoring-prometheusext-operator
    - name: ibm-healthcheck-operator
    - name: ibm-management-ingress-operator
    - name: ibm-ingress-nginx-operator
    - name: ibm-metering-operator
    - name: ibm-licensing-operator
    - name: ibm-commonui-operator
    - name: ibm-auditlogging-operator
    - name: ibm-platform-api-operator
```

After the `OperandRequest` created, we can check if our common services install successfully by command.

```bash
kubectl -n ibm-common-services get csv
```

### Enable or Delete an Operator

- Enable an operator, you can add the operator into the `OperandRequest`

- Delete an operator, you can remove the operator from the`OperandRequest`

## Post-installation

The operators and their custom resource would be deployed in the cluster, and thus the installation of operands will also triggered by the CR.