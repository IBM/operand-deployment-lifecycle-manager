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
  - [OperandBindInfo Spec](#operandbindinfo-spec)
  - [E2E Use Case](#e2e-use-case)
  - [Operator/Operand Upgrade](#operatoroperand-upgrade)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Operand Deployment Lifecycle Manager (ODLM)

Operand is the instance managed by the operator. ODLM is used to manage the lifecycle of for a group of operands, compared with operator lifecycle manager, ODLM focus on the management of operands but not operators.

The ODLM will have four CRDs:

| Resource                 | Short Name | Description                                                                                |
|--------------------------|------------|--------------------------------------------------------------------------------------------|
| OperandRequest | opreq | It defines which operator/operand want to be installed in the cluster |
| OperandRegistry | opreg | It defines the OLM information, like channel and catalog source, for each operator |
| OperandConfig | opcon | It defines the parameters that should be used to install the operator's operand |
| OperandBindInfo | opbi | It identifies secrets and/or configmaps that should be shared with requests |

## Goal

1. A single entrypoint to manage a group of operands
1. User can select a set of operands to install
1. The install can be invoked through either OCP UI or CLI

## ODLM Workflow

![w](../images/odlm-arch.png)


## OperandRegistry Spec

OperandRegistry defines the OLM information, like channel and catalog source, for each operator.

**NOTE:** When the ODLM operator is deployed, it generates a default OperandRegistry instance. You can edit the instance as required.

Following is an example of the OperandRegistry CR:

**NOTE:** The "name" parameter must be unique for each entry.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: OperandRegistry
metadata:
  name: example-service [1]
  namespace: example-service-ns [2]
spec:
  operators:
  - name: jenkins [3]
    namespace: default [4]
    channel: alpha [5]
    packageName: jenkins-operator [6]
    scope: public [7]
    sourceName: community-operators [8]
    sourceNamespace: openshift-marketplace [9]
```

The Operand (Deployment) Registry Custom Resource (CR) lists OLM Operator information for operands that may be requested for installation and/or access by an application that runs in a namespace. The registry CR specifies:

1. `name` of the OperandRegistry
2. `namespace` of the OperandRegistry
3. `name` is the name of the operator, which should be the same as the services name in the OperandConfig and OperandRequest.
4. `namespace` is the namespace where the operator will be deployed.
5. `channel` is the name of OLM channel that is subscribed for the operator.
6. `packageName` is the name of the package in CatalogSource that is subscribed for the operator.
7. `scope` is an indicator, either public or private, that dictates whether deployment can be requested from other namespaces (public) or only from the containing names (private). The default is private.
8. `sourceName` is the name of the CatalogSource.
9. `sourceNamespace` is the namespace of the CatalogSource.

## OperandConfigs Spec

OperandConfig defines the individual operand deployment configuration. The Operand Config Custom Resource (CR) defines the parameters for each operator that is listed in the OperandRegistry that should be used to install the operator instance by specifying an installation CR.

**NOTE:** When ODLM operator is deployed, it generates a default OperandConfig instance. You can edit the instance as required.

```yaml
apiVersion: operator.ibm.com/v1alpha1
Kind: OperandConfigs
metadata:
  name: example-service [1]
  namespace: example-service-ns [2]
spec:
  services:
  - name: jenkins [3]
    spec: [4]
      jenkins:
        port: 8081
```

OperandConfig defines the individual operand deployment config:

1. `name` of the OperandConfig
2. `namespace` of the OperandConfig
3. `name` is the name of the operator, which should be the same as the services name in the OperandRegistry and OperandRequest.
4. `spec` defines a map. Its key is the kind name of the custom resource. Its value is merged to the spec field of custom resource. For more details, you can check the following topic How does ODLM create the individual operator CR?

### How does Operator create the individual operator CR?

Jenkins Operator has one CRD: Jenkins:

The OperandConfig CR has

```yaml
- name: jenkins
  spec:
    jenkins:
      service:
        port: 8081
```

The IAM Operator CSV has

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  annotations:
   alm-examples: |-
    [
     {
       "apiVersion":"jenkins.io/v1alpha2",
       "kind":"Jenkins",
       "metadata": {
         "name":"example"
       },
       "spec":{
         ...
         "service":{"port":8080,"type":"ClusterIP"},
         ...
       }
     }
  ]
```

The ODLM will deep merge the OperandConfig CR spec and Jenkins Operator CSV alm-examples to create the Jenkins CR.

```yaml
apiVersion: jenkins.io/v1alpha2
kind: Jenkins
metadata:
  name: example
spec:
  ...
  service:
    port: 8081
    type: ClusterIP
  ...
```

For day2 operations, the ODLM will patch the OperandConfigs CR spec to the existing Jenkins CR.

Typically, users update the individual operator CR for day2 operations, but OperandConfig still provides the ability for individual operator/operand day2 operation.

## OperandRequests Spec

OperandRequest defines which operator/operand you want to install in the cluster.

**NOTE:** OperandRequest custom resource is used to trigger a deployment for Operators and Operands.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: OperandRequest
metadata:
  name: example-service [1]
  namespace: example-service-ns [2]
spec:
  requests:
  - registry: example-service [3]
    registryNamespace: example-service-ns [4]
    operands: [5]
    - name: jenkins [6]
      bindings: [7]
        public:
          secret: jenkins-operator-credential [8]
          configmap: jenkins-operator-base-configuration [9]
```

1. `name` of the OperandRequest
2. `namespace` of the OperandRequest
3. `registry` identifies the name of the OperandRegistry CR from which this operand deployment is being requested.
4. `registryNamespace` identifies the namespace in which the catalog CR is defined. Note: If the catalog name and namespace are not specified then it is assumed that the Request (1) is for a catalog in the current (requester's) namespace and (2) that only one catalog exists in the namespace.
5. `operands` in the CR is a list of operands.
6. `name` of operands in the CR must match a name specification in an OperandRegistry's CR.
7. The `bindings` of the operands is a map to get and rename the secret and/or configmap from the provider and creat them in the requester's namespace. If the requester wants to rename the secret and/or configmap, they need to know the key of the binding in the OperandBindInfo. If the key of the bindings map is prefixed with public, it means the secret and/or configmap can be shared with the requester in the other namespace. If the key of the bindings map is prefixed with private, it means the secret and/or configmap can only be shared within its own namespace.
8. The optional `secret` names a secret that should be created in the requester's namespace with formatted data that can be used to interact with the service.
9. The optional `configmap` field names a configmap that should be created in the requester's namespace with formatted data that can be used to interact with the service.

## OperandBindInfo Spec

The ODLM will use the OperandBindInfo to copy the generated secret and/or configmap to a requester's namespace when a service is requested with the OperandRequest CR. An example specification for an OperandBindInfo CR is shown below.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: OperandBindInfo
metadata:
  name: publicjenkinsbinding [1]
  namespace: example-service-ns [2]
spec:
  operand: jenkins [3]
  registry: example-service [4]
  description: "Binding information that should be accessible to jenkins adopters" [5]
  bindings: [6]
    public:
      secret: jenkins-operator-credentials-example [7]
      configmap: jenkins-operator-base-configuration-example [8]
```

Fields in this CR are described below.

1. `name` of the OperandBindInfo
2. `namespace` of the OperandBindInfo
3. The `operand` should be the the individual operator name.
4. The `registry` section must match the name in the OperandRegistry in the current namespace.
5. `description` is used to add a detailed description of a service.
6. The `bindings` section is used to specify information about the access/configuration data that is to be shared. If the key of the bindings map is prefixed with public, it means the secret and/or configmap can be shared with the requester in the other namespace. If the key of the bindings map is prefixed with private, it means the secret and/or configmap can only be shared within its own namespace.
7. The `secret` field names an existing secret, if any, that has been created and holds information that is to be shared with the adopter/requester.
8. The `configmap` field identifies a configmap object, if any, that should be shared with the adopter/requester

ODLM will use the OperandBindInfo CR to pass information to an adopter when they create a OperandRequest to access the service, assuming that both have compatible scopes. ODLM will copy the information from the shared service's "OperandBindInfo.bindinfo[].secret" and/or "OperandBindInfo.bindinfo[].configmap" to the requester namespace.

**NOTE:** If in the OperandRequest, there is no secret and/or configmap name specified in the bindings or no bindings field in the element of operands, ODLM will copy the secret and/or configmap to the requester's namespace and rename them to the name of the OperandBindInfo + secret/configmap name.

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
