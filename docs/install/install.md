<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Install the operand deployment lifecycle manager On OCP 4.3+](#install-the-operand-deployment-lifecycle-manager-on-ocp-43)
  - [1. Create CatalogSource](#1-create-catalogsource)
  - [2. Create Operator NS, Group, Subscription](#2-create-operator-ns-group-subscription)
  - [3. Check Operator CSV](#3-check-operator-csv)
  - [4. Create OperandRegistry and OperandConfig instance](#4-create-operandregistry-and-operandconfig-instance)
    - [Create OperandConfig](#create-operandconfig)
    - [Create OperandRegistry](#create-operandregistry)
  - [5. Create OperandRequest instance](#5-create-operandrequest-instance)
    - [Create Operand Request](#create-operand-request)
    - [Enable or Delete an Operator](#enable-or-delete-an-operator)
  - [Post-installation](#post-installation)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Install the operand deployment lifecycle manager On OCP 4.3+

## 1. Create CatalogSource

```yaml
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
```

## 2. Create Operator NS, Group, Subscription

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

## 3. Check Operator CSV

```bash
oc -n ibm-common-services get csv
```

## 4. Create OperandRegistry and OperandConfig instance

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
