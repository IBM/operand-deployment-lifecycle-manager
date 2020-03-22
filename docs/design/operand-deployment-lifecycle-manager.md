<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Operand Deployment Lifecycle Manager (ODLM)](#operand-deployment-lifecycle-manager-odlm)
  - [Goal](#goal)
  - [ODLM Workflow](#odlm-workflow)
  - [OperandRegistry Spec](#operandregistry-spec)
  - [OperandConfigs Spec](#operandconfigs-spec)
    - [How does Operator create the individual operator CR?](#how-does-operator-create-the-individual-operator-cr)
  - [OperandRequests Spec](#operandrequests-spec)
    - [Data Sharing Between Different Operands](#data-sharing-between-different-operands)
  - [E2E Use Case](#e2e-use-case)
  - [Operator/Operand Upgrade](#operatoroperand-upgrade)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Operand Deployment Lifecycle Manager (ODLM)

Operand is the instance managed by the operator. ODLM is used to manage the lifecycle of for a group of operands, compared with operator lifecycle manager, ODLM focus on the management of operands but not operators.

The ODLM will have Three CRDs:

* OperandRegistry, defines the individual operand deployment info
* OperandConfigs, defines the individual operand deployment config
* OperandRequests, defines which operator/operand want to be installed in the cluster

## Goal

1. A single entrypoint to manage a group of operands
1. User can select a set of operands to install
1. The install can be invoked through either OCP UI or CLI

## ODLM Workflow

![w](../images/odlm-arch.png)


## OperandRegistry Spec

OperandRegistry defines the operand deployment info:

* namespace, the namespace of the individual operand
* channel, the channel(version) of the individual operand
* packageName, the package name in operator registry of the individual operand
* sourceName, the name of operator registry
* sourceNamespace, the namespace of operator registry

The OperandRegistry managed operator subscriptions will have a label `"operator.ibm.com/opreq-control": "true"`, we use this label to distinguish if an operator is managed by OperandRegistry or not.

> **NOTE: These operands will not be installed by default, unless you explicitly set them in OperandRequests**

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: OperandRegistry
metadata:
  name: common-service
spec:
  operators:
  - name: mongodb
    namespace: ibmcs-mongodb
    channel: stable-3.3
    packageName: mongodb
    sourceName: ibmcloud-operators
    sourceNamespace: openshift-marketplace
  - name: iam
    namespace: ibmcs-iam
    channel: stable-3.3
    packageName: iam
    sourceName: ibmcloud-operators
    sourceNamespace: openshift-marketplace
  - name: kui
    namespace: ibmcs-kui
    channel: stable-3.4
    packageName: kui
    sourceName: ibmcloud-operators
    sourceNamespace: openshift-marketplace
  - name: metering
    namespace: ibmcs-metering
    channel: stable-3.5
    packageName: metering
    sourceName: ibmcloud-operators
    sourceNamespace: openshift-marketplace
```

## OperandConfigs Spec

OperandConfigs defines the individual operand deployment config:

* name, the individual operand name
* spec, the parameters that used to generate the CR

```yaml
apiVersion: operator.ibm.com/v1alpha1
Kind: OperandConfigs
metadata:
  name: common-service
spec:
  services:
  # suppose mongodb has one CRD
  - name: mongodb
    spec:
      mongodb:
        storageClass: rook-ceph
  # suppose iam has two CRDs
  - name: iam
    spec:
      apikey:
        key: value
        nested:
          key: value
      identity:
        key: value
  # suppose metering has three CRDs
  - name: metering
    spec:
      reader:
        key: value
        nested:
          key: value
      server:
        key: value
        nested:
          key: value
      dataManager:
        key: value
        nested:
          key: value
  - name: monitoring
    spec:
      monitor: {}
```

### How does Operator create the individual operator CR?

Suppose IAM Operator has two CRDs: Apikey and Identity:

1. The OperandConfigs CR has

    ```yaml
    - name: iam
      spec:
        apikey:
          key: value
          nested:
            key: value
        identity:
          key: value
    ```

2. The IAM Operator CSV has

    ```yaml
    apiVersion: operators.coreos.com/v1alpha1
    kind: ClusterServiceVersion
    metadata:
      annotations:
        alm-examples: |-
          [
            {
              "apiVersion": "iam.operator.ibm.com/v1alpha1",
              "kind": "Apikey",
              "metadata": {
                "name": "iam-apikey"
              },
              "spec": {
                "key": "value",
                "nested": {
                  "key": "value"
                }
              }
            },
            {
              "apiVersion": "iam.operator.ibm.com/v1alpha1",
              "kind": "Identity",
              "metadata": {
                "name": "iam-identity"
              },
              "spec": {
                "key": "value"
                }
              }
            }
          ]
    ```

3. The ODLM will deep merge the OperandConfigs CR spec and IAM Operator CSV alm-examples to create the IAM CR.

4. For day2 operations, the ODLM will patch the OperandConfigs CR spec to the existing IAM CR.

    Usually the user should update the individual operator CR for day2 operations, but OperandConfigs still provide the ability for individual operator/operand day2 operation.


## OperandRequests Spec

OperandRequests defines which operator/operand want to be installed in the cluster.

* name, the name of the individual operator
* channel, optional parameter, the subscription channel(version) of the individual operator
* state, the state of the individual operator

```yaml
apiVersion: operator.ibm.com/v1alpha1
Kind: OperandRequest
metadata:
  name: common-service
spec:
  services:
  - name: iam
    catalog: common-services
    catalognamespace: common-services  
  - name: mongo
    catalog: common-services
    catalognamespace: common-services
```


### Data Sharing Between Different Operands

The producer operand can generate a configMap or secrets, and enable other operands have permission to read those configMap and secrets. This can be achieved via `role` and `rolebinding`.

An example as follows. All of the service accounts under namespace `ibm-common-services` are able to read the secrets and configMap for MongoDB.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: ibm-mongodb-operator-info
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  resourceNames:
  - icp-mongodb-client-cert
  - icp-mongodb-admin
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - configmaps
  resourceNames:
  - icp-mongodb
  verbs:
  - get

  ```

  ```yaml
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: ibm-mongodb-operator-info
subjects:
- kind: Group
  name: system:serviceaccounts:ibm-common-services
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: ibm-mongodb-operator-info
  apiGroup: rbac.authorization.k8s.io
  ```

## E2E Use Case

1. User installs ODLM from OLM

    The ODLM will automatically generate two default CRD CRs, since OperandReigstrt and OperandConfigs CRs don't define the state, so it should be fine.

2. Optional: User update the OperandConfigs CR with their own parameter values

    This should be optional if the default configurations match the requirement.

3. User creates the OperandRequest CR from OLM

    This tells ODLM that users want to install some of the individual operator/operand.

4. The rest works will be done by OLM and ODLM

    And finally user will get what they want.

## Operator/Operand Upgrade

- For operator/operand upgrade, you only need to publish your operator OLM to your operator channel, and OLM will handle the upgrade automatically.
- If there are major version, then you may want to update `channel` in `OperandRegistry` to trgger upgrade.
