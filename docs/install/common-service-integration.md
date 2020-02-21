<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Pre-Requisites](#pre-requisites)
  - [Check your operator bundle](#check-your-operator-bundle)
  - [Push the operator to quay.io](#push-the-operator-to-quayio)
  - [Verify your operator on the Openshift console](#verify-your-operator-on-the-openshift-console)
    - [1. Create OperatorSource](#1-create-operatorsource)
    - [2. Create a Namespace for your operator](#2-create-a-namespace-for-your-operator)
    - [3. Install Operator](#3-install-operator)
- [Common Service Onboarding](#common-service-onboarding)
  - [1. Clone the git repository of meta operator](#1-clone-the-git-repository-of-meta-operator)
  - [2. Edit default value of custom resource](#2-edit-default-value-of-custom-resource)
    - [Edit the MetaOperator Catalog custom resource](#edit-the-metaoperator-catalog-custom-resource)
    - [Edit the MetaOperator Config custom resource](#edit-the-metaoperator-config-custom-resource)
    - [Edit a MetaOperator Set custom resource](#edit-a-metaoperator-set-custom-resource)
  - [3.Make a pull request to merge the changes](#3make-a-pull-request-to-merge-the-changes)
- [End to end test](#end-to-end-test)
  - [1. Create an OperatorSource in the Openshift cluster](#1-create-an-operatorsource-in-the-openshift-cluster)
  - [2. Create a Namespace `meta-operator`](#2-create-a-namespace-meta-operator)
  - [3. Install meta Operator](#3-install-meta-operator)
  - [4. Check the installed operators](#4-check-the-installed-operators)
  - [5. Edit the MetaOperator Config custom resource and the MetaOperator Catalog custom resource](#5-edit-the-metaoperator-config-custom-resource-and-the-metaoperator-catalog-custom-resource)
  - [6. Create a MetaOperator Set](#6-create-a-metaoperator-set)
  - [7. Check the installed operators and their custom resource](#7-check-the-installed-operators-and-their-custom-resource)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Pre-Requisites

## Check your operator bundle

- [operator-courier](https://github.com/operator-framework/operator-courier)
- [scorecard](https://github.com/operator-framework/operator-sdk/blob/master/doc/test-framework/scorecard.md)

## Push the operator to quay.io

All common services OLMs and operator images should be published in public org in Quay.io: [OpenCloudio](https://quay.io/organization/opencloudio)

more information see the [push olm to quay.io](https://github.com/operator-framework/community-operators/blob/master/docs/testing-operators.md#push-to-quayio).

## Verify your operator on the Openshift console

### 1. Create OperatorSource

Open OCP console, click the `Plus` button on the top right and paste the following content, then click `Create`.

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

### 2. Create a Namespace for your operator

Open the `OperatorHub` page in OCP console left menu, then `Create Project`.

### 3. Install Operator

Open `OperatorHub`, search your operator and install it.

# Common Service Onboarding

## 1. Clone the git repository of meta operator

```bash
git clone git@github.com:IBM/meta-operator.git
```

## 2. Edit default value of custom resource

### Edit the MetaOperator Catalog custom resource

```bash
cd meta-operator
vi deploy/crds/operator.ibm.com_v1alpha1_metaoperatorcatalog_cr.yaml
```

Append the operator package information under the `operators` field.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: MetaOperatorCatalog
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
    description: The jenkins service depends on ...
...
```

- `name` is the name of the operator, which should be the same as the services name in the `MetaOperatorConfig` and `MetaOperatorSet`.
- `namespace` is the namespace the operator will be deployed in. For the common services, they need to be installed in the `ibm-common-services` namespace.
- `channel` is the name of a tracked channel.
- `packageName` is the name of the package in `CatalogSource` will be deployed.
- `sourceName` is the name of the `CatalogSource`.
- `sourceNamespace` is the namespaces of the `CatalogSource`.
- `description` is used to add a detailed description for service including clarifying the dependency.

### Edit the MetaOperator Config custom resource

```bash
cd meta-operator
vi deploy/crds/operator.ibm.com_v1alpha1_metaoperatorconfig_cr.yaml
```

Append the operator custom resource information under the `services` field.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: MetaOperatorConfig
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
- The `spec` field defines a map. Its key `jenkins` is the kind name of the custom resource. Its value `service.port: 8081` will be merged to the `spec` field of custom resource `jenkins`.

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

**Note:** When onboarding your operator, you need to put all the default configuration into the `alm-example`of your CSV and leave the value of map in `spec` to `{}`.
For example:

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: MetaOperatorConfig
metadata:
  name: common-service
spec:
  services:
  ...
  - name: jenkins
    spec:
      jenkins: {}
  ...
```

### Edit a MetaOperator Set custom resource

```bash
cd meta-operator
vi deploy/crds/operator.ibm.com_v1alpha1_metaoperatorset_cr.yaml
```

Append the operator information under the `services` field.

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: MetaOperatorSet
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

- `services` is a list defines the set for each service.
- `name` is the service name, which should be the same as the services name in the `MetaOperatorConfig` and operator name in the `MetaOperatorCatalog`.
- `channel` is an optional setting, it can overwrite the `channel` defined in the `MetaOperatorCatalog`.
- `state` defines if the service should be present or absent.
- `description` is the description of the service.

## 3.Make a pull request to merge the changes

# End to end test

**Note:** before running the e2e test, users have to push your own CSV package to Quay.io.
more information see [Push the operator to quay.io](#push-the-operator-to-quayio).

## 1. Create an OperatorSource in the Openshift cluster

Open OCP console, click the `Plus` button on the top right and paste the following content, then click `Create`.

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

## 2. Create a Namespace `meta-operator`

Open `OperatorHub` page in OCP console left menu, then `Create Project`, e.g., create a project named `meta-operator`.

## 3. Install meta Operator

Open `OperatorHub` and search `meta-operator` to find the operator, and install it.

## 4. Check the installed operators

Open `Installed Operators` page to check the installed operators.

## 5. Edit the MetaOperator Config custom resource and the MetaOperator Catalog custom resource

- [Editing MetaOperator Config](#edit-meta-operator-config-custom-resource)
- [Editing MetaOperator Catalog](#edit-meta-operator-custom-resource)

## 6. Create a MetaOperator Set

```bash
vi deploy/crds/operator.ibm.com_v1alpha1_metaoperatorset_cr.yaml
oc apply -f deploy/crds/operator.ibm.com_v1alpha1_metaoperatorset_cr.yaml -n meta-operator
```

- [Editing MetaOperator Set](#edit-meta-operator-set-custom-resource)

## 7. Check the installed operators and their custom resource
