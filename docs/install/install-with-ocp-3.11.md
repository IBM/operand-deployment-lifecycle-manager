<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Install the operand deployment lifecycle manager On OCP 3.11](#install-the-operand-deployment-lifecycle-manager-on-ocp-311)
  - [0. Install OLM](#0-install-olm)
  - [1. Build Operator Registry image](#1-build-operator-registry-image)
  - [2. Create CatalogSource](#2-create-catalogsource)
  - [3. Create Operator NS, Group, Subscription](#3-create-operator-ns-group-subscription)
  - [4. Check Operator CSV](#4-check-operator-csv)
  - [5. Create OperandRequest instance](#5-create-operandrequest-instance)
    - [Create Operand Request](#create-operand-request)
    - [Enable or Delete an Operator](#enable-or-delete-an-operator)
  - [Post-installation](#post-installation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Install the operand deployment lifecycle manager On OCP 3.11

## 0. Install OLM

```bash
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.13.0/install.sh | bash -s 0.13.0
```

## 1. Build Operator Registry image

> You need to remove the last `type: object` in all CRDs to avoid the following error: The CustomResourceDefinition "operandconfigs.operator.ibm.com" is invalid: spec.validation.openAPIV3Schema: Invalid value: apiextensions.JSONSchemaProps ..... must only have "properties", "required" or "description" at the root if the status subresource is enabled

```bash
cd deploy
docker build -t quay.io/opencloudio/operator-registry -f operator-registry.Dockerfile .
docker push quay.io/opencloudio/operator-registry
```

## 2. Create CatalogSource

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: opencloud-operators
  namespace: olm
spec:
  sourceType: grpc
  image: docker.io/ibmcom/ibm-common-service-catalog:latest
```

## 3. Create Operator NS, Group, Subscription

```yaml
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
```

## 4. Check Operator CSV

```bash
oc -n ibm-common-services get csv
```

## 5. Create OperandRequest instance

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
