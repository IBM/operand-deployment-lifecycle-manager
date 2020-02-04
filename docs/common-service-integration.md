<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Pre-Requisites](#pre-requisites)
    - [Push the operator to quay.io](#push-the-operator-to-quayio)
- [Integration services with Common Service operator](#integration-services-with-common-service-operator)
    - [1. Fork the git repository of Common Service operator](#1-fork-the-git-repository-of-common-service-operator)
    - [2. Edit default value of custom resource](#2-edit-default-value-of-custom-resource)
        - [Edit MetaOperator custom resource](#edit-metaoperator-custom-resource)
        - [Edit Common Service Config custom resource](#edit-common-service-config-custom-resource)
        - [Edit Common Service Set custom resource](#edit-common-service-set-custom-resource)
    - [3.Make a pull request to merge the changes](#3make-a-pull-request-to-merge-the-changes)
- [End to end test](#end-to-end-test)
    - [1. Create OperatorSource in the Openshift cluster](#1-create-operatorsource-in-the-openshift-cluster)
    - [2. Create a Namespace `common-service-operator`](#2-create-a-namespace-common-service-operator)
    - [3. Install Common Service Operator](#3-install-common-service-operator)
    - [4. Check the installed operators](#4-check-the-installed-operators)
    - [5. Edit Common Service Config and MetaOperator custom resource](#5-edit-common-service-config-and-metaoperator-custom-resource)
    - [6. Create Common Service Set](#6-create-common-service-set)
    - [7. Check the installed operators and their custome resource](#7-check-the-installed-operators-and-their-custome-resource)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Pre-Requisites

## Push the operator to quay.io

All common services OLMs and operator images should be published in public org in Quay.io: [OpenCloudio](https://quay.io/organization/opencloudio)

more information see the [push olm to quay.io](https://github.com/operator-framework/community-operators/blob/master/docs/testing-operators.md#push-to-quayio).

# Integration services with Common Service operator

## 1. Fork the git repository of Common Service operator

## 2. Edit default value of custom resource

### Edit MetaOperator custom resource

```bash
cd common-service-operator
vi deploy/crds/operator.ibm.com_v1alpha1_metaoperator_cr.yaml
```

Append the operator package information under `operators` field.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: MetaOperator
metadata:
name: common-service
spec:
operators:
...
- name: jenkins
    namespace: jenkins-operator
    channel: alpha
    packageName: jenkins-operator
    sourceName: community-operators
    sourceNamespace: openshift-marketplace
    targetNamespaces:
    - jenkins-operator
...
```

- `name` is the name of the operator, which should be same as the services name in the `CommonServiceConfig` and `CommonServiceSet`.
- `namespace` is the namespace the operator will be deployed in.
- `channel` is the name of tracked channel.
- `packageName` is the name of the package in `CatalogSource` will be deployed.
- `sourceName` is the name of the `CatalogSource`.
- `sourceNamespace` is the namespace of the `CatalogSource`.
- `targetNamespaces` is a list of namespace, which `OperaterGroup` generates RBAC access for its member Operators to get access to. `targetNamespaces` is used to control the operator dependency. `targetNamespaces` should include all the namespaces of its dependent operators and its own namespace.

### Edit Common Service Config custom resource

```bash
cd common-service-operator
vi deploy/crds/operator.ibm.com_v1alpha1_commonserviceconfig_cr.yaml
```

Append the operator custom resource information under `services` field.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: CommonServiceConfig
metadata:
  name: common-service
spec:
  services:
  ...
  - name: jenkins
    spec:
      jenkins:
        service:
          port: 8081
  ...
```

Take the jenkins operator as an example.
- The `name` field defines the name of the operator.
- The `spec` field defines the `spec` configuration for each custom resource in the service.

In this example:
The configuration for custom resource `Jenkins` is:

```yaml
      jenkins:
        service:
          port: 8081
```

The configuration will be merged in the `spec` of the relevant `alm-example` in the `cluster service version` and generate custom resource `Jenkins`

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

### Edit Common Service Set custom resource

```bash
cd common-service-operator
vi deploy/crds/operator.ibm.com_v1alpha1_commonserviceset_cr.yaml
```

Append the operator information under `services` field.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: CommonServiceSet
metadata:
  name: common-service
spec:
  services:
  ...
  - name: jenkins
    channel: alpha
    state: present
    description: The jenkins service
  ...
```

-  `services` is a list defines the set for each service.
- `name` is the service name, which should be the same as the services name in the `CommonServiceConfig` and operator `name` in the `MetaOperator`.
- `channel` is an optional setting, it can overwrite the `channel` defined in the `MetaOperator`.
- `state` defines if the service should be present or absent.
- `description` is the description of the service.

## 3.Make a pull request to merge the changes

# End to end test

**Note:** before run the e2e test, user have to push your own CSV package to source.
more information see [Push the operator to quay.io](#push-the-operator-to-quayio).

## 1. Create OperatorSource in the Openshift cluster

Open OCP console, click the `Plus` button on top right and paste following content, then click `Create`.

```yaml
apiVersion: operators.coreos.com/v1
kind: OperatorSource
metadata:
  name: opencloud-operators
  namespace: openshift-marketplace
spec:
  authorizationToken: {}
  displayName: IBMCS Operators
  endpoint: https://quay.io/cnr
  publisher: IBM
  registryNamespace: opencloudio
  type: appregistry
```

## 2. Create a Namespace `common-service-operator`

Open `OperatorHub` page in OCP console left menu, then `Create Project`, e.g., create a project named as `common-service-operator`.

## 3. Install Common Service Operator

Open `OperatorHub` and search `common-service-operator` to find the operator, and install it.

## 4. Check the installed operators

Open `Installed Operators` page to check the installed operators.

## 5. Edit Common Service Config and MetaOperator custom resource

- [Common Service Config](#edit-common-service-config-custom-resource)
- [MetaOperator](#edit-meta-operator-custom-resource)

## 6. Create Common Service Set

```bash
vi deploy/crds/operator.ibm.com_v1alpha1_commonserviceconfig_cr.yaml
oc apply -f deploy/crds/operator.ibm.com_v1alpha1_commonserviceconfig_cr.yaml -n common-service-operator
```

- [Common Service Set](#edit-common-service-set-custom-resource)

## 7. Check the installed operators and their custome resource
