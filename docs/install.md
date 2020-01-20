# Install

## On OCP 4.x

### 1. Create OperatorSource

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

### 2. Create a Namespace `common-service-operator`

    Open `OperatorHub` page in OCP console left menu, then `Create Project`, the name MUST be `common-service-operator`.

### 3. Install Common Service Operator

    Open `OperatorHub` and search `common-service-operator` to find the operator, and install it.

### 4. Check the installed operators

    Open `Installed Operators` page to check the installed operators.

## On OCP 3.11

### 0. Install OLM

    ```bash
    curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.13.0/install.sh | bash -s 0.13.0
    ```

### 1. Build Operator Registry image

    > You need to remove the last `type: object` in all CRDs to avoid following error: The CustomResourceDefinition "commonserviceconfigs.operator.ibm.com" is invalid: spec.validation.openAPIV3Schema: Invalid value: apiextensions.JSONSchemaProps ..... must only have "properties", "required" or "description" at the root if the status subresource is enabled

    ```bash
    cd deploy
    docker build -t quay.io/opencloudio/operator-registry -f operator-registry.Dockerfile .
    docker push quay.io/opencloudio/operator-registry
    ```

### 2. Create CatalogSource

    ```yaml
    apiVersion: operators.coreos.com/v1alpha1
    kind: CatalogSource
    metadata:
      name: opencloud-operators
      namespace: olm
    spec:
      sourceType: grpc
      image: quay.io/opencloudio/operator-registry:latest
    ```

### 3. Create Operator NS, Group, Subscription

    ```yaml
    apiVersion: v1
    kind: Namespace
    metadata:
      name: common-service-operator

    ---
    apiVersion: operators.coreos.com/v1alpha2
    kind: OperatorGroup
    metadata:
      name: operatorgroup
      namespace: common-service-operator
    spec:
      targetNamespaces:
      - common-service-operator

    ---
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: common-service
      namespace: common-service-operator
    spec:
      channel: alpha
      name: common-service
      source: opencloud-operators
      sourceNamespace: olm
    ```

### 4. Check Operator CSV

    ```bash
    oc -n common-service-operator get csv
    ```
