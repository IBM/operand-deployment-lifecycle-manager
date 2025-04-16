<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Operand Deployment Lifecycle Manager (ODLM)](#operand-deployment-lifecycle-manager-odlm)
  - [What are operands?](#what-are-operands)
  - [Goal](#goal)
  - [ODLM Workflow](#odlm-workflow)
  - [OperandRegistry Spec](#operandregistry-spec)
  - [OperandConfig Spec](#operandconfig-spec)
    - [How does Operator create the individual operator CR](#how-does-operator-create-the-individual-operator-cr)
  - [OperandRequest Spec](#operandrequest-spec)
    - [OperandRequest sample to create custom resource via OperandConfig](#operandrequest-sample-to-create-custom-resource-via-operandconfig)
    - [OperandRequest sample to create custom resource via OperandRequest](#operandrequest-sample-to-create-custom-resource-via-operandrequest)
  - [OperandBindInfo Spec](#operandbindinfo-spec)
  - [E2E Use Case](#e2e-use-case)
  - [Operator/Operand Upgrade](#operatoroperand-upgrade)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Operand Deployment Lifecycle Manager (ODLM)

The operand is the instance managed by the operator. ODLM is used to manage the lifecycle of for a group of operands, compared with operator lifecycle manager, ODLM focus on the management of operands but not operators.

## What are operands?

Operator is a method of packaging, deploying and managing a Kubernetes application.
Operands are the services and applications that Operator manage.

For example, cert-manager operator deploys a cert-manager deployment, then the cert-manager-operator is an operator and the cert-manager deployment is an operand.

The ODLM will have four CRDs:

| Resource                 | Short Name | Description                                                                                |
|--------------------------|------------|--------------------------------------------------------------------------------------------|
| OperandRequest | opreq | It defines which operator/operand want to be installed in the cluster |
| OperandRegistry | opreg | It defines the OLM information, like channel and catalog source, for each operator |
| OperandConfig | opcon | It defines the parameters that should be used to install the operator's operand |
| OperandBindInfo | opbi | It identifies secrets and/or configmaps that should be shared with requests |

## Goal

1. A single entry point to manage a group of operands
1. User can select a set of operands to install
1. The install can be invoked through either OCP UI or CLI

## ODLM Workflow

![w](../images/odlm-arch.png)

## OperandRegistry Spec

OperandRegistry defines the OLM information used for installation, like package name and catalog source, for each operator.

Following is an example of the OperandRegistry CR:

**NOTE:** The "spec.operators[*].name" parameter must be unique for each entry.

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
    installMode: cluster [10]
    installPlanApproval: Manual [11]
```

The OperandRegistry Custom Resource (CR) lists OLM Operator information for operands that may be requested for installation and/or access by an application that runs in a namespace. The registry CR specifies:

1. `name` of the OperandRegistry
2. `namespace` of the OperandRegistry
3. `name` is the name of the operator, which should be the same as the services name in the OperandConfig and OperandRequest.
4. (optional) `namespace` defines the namespace where the operator will be deployed. (1) When InstallMode is `cluster`, the operator will be deployed into the `openshift-operators` namespace. (2) When InstallMode is empty or set to `namespace`, it is the namespace where operator will be deployed. (3) If the `namespace` value is empty, the operator will be deployed in the same namespace as this OperandRegistry.
5. `channel` is the name of OLM channel that is subscribed for the operator.
6. `packageName` is the name of the package in CatalogSource that is subscribed for the operator.
7. (optional) `scope` is an indicator, either public or private, that dictates whether deployment can be requested from other namespaces (public) or only from the namespace of this OperandRegistry (private). The default value is private.
8. `sourceName` is the name of the CatalogSource.
9. `sourceNamespace` is the namespace of the CatalogSource.
10. (optional) `installMode` is the install mode of the operator, can be `namespace` (OLM one namespace), `cluster` (OLM all namespaces) or `no-op` (discontinued service). The default value is `namespace`. Operator is deployed in `openshift-operators` namespace when InstallMode is set to `cluster`.
11. (optional) `installPlanApproval` is the approval mode for emitted installplan. The default value is `Automatic`.

## OperandConfig Spec

OperandConfig defines the individual operand configuration. The OperandConfig Custom Resource (CR) defines the parameters for each operator that is listed in the OperandRegistry that should be used to install the operator instance by specifying an installation CR.

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
4. `spec` defines a map. Its key is the kind name of the custom resource. Its value is merged to the spec field of custom resource. For more details, you can check the following topic **How does ODLM create the individual operator CR?**

### Dynamic Configuration with Templating

OperandConfig supports a templating system that allows values to be dynamically referenced from various sources rather than hardcoding them in the CR. This enables more flexible configuration management and helps maintain separation between configuration data and its sources.

The templating system uses the `templatingValueFrom` field to reference values from ConfigMaps, Secrets, or other Kubernetes objects. When ODLM processes an OperandConfig, it resolves these references to their actual values before creating the operand resources.

**Example:** Templating a Jenkins service port using a ConfigMap value.

```yaml
  apiVersion: operator.ibm.com/v1alpha1
  Kind: OperandConfigs
  metadata:
    name: common-service
    namespace: service-ns
  spec:
    services:
    - name: keycloak-operator
      resources:
        - apiVersion: v1
            data:
              stringData:
                ca.crt:
                  templatingValueFrom:
                    configMapKeyRef:
                      key: service-ca.crt
                      name: openshift-service-ca.crt
                    required: true
              type: kubernetes.io/tls
            force: true
            kind: Secret
            name: cs-keycloak-tls-secret
```
In this example, the `ca.crt` field in the Secret is populated from a key `service-ca.crt` in a ConfigMap named `openshift-service-ca.crt`. The `templatingValueFrom` field specifies how to retrieve the value, and the `required: true` flag indicates that this value must be present for the configuration to be valid.

#### Value Source Types

ODLM supports several types of value sources that can be referenced using templatingValueFrom:

1. ConfigMap Reference (`configMapKeyRef`)
    Values can be retrieved from specific keys within ConfigMaps. This is useful for environment-specific configuration that might change between deployments.

    ```yaml
    templatingValueFrom:
    configMapKeyRef:
      name: my-configmap        # Name of the ConfigMap
      namespace: my-namespace  # Optional: Defaults to OperandConfig namespace
      key: data.key            # Key within the ConfigMap's data field
    # required: true           # Optional: Defaults to false
    ```

2. Secret Reference (`secretKeyRef`)
    For sensitive data like passwords, API tokens, or certificates, values can be securely retrieved from specific keys within Secrets.

    ```yaml
    templatingValueFrom:
      secretKeyRef:
        name: my-secret           # Name of the Secret
        namespace: my-namespace  # Optional: Defaults to OperandConfig namespace
        key: data.key            # Key within the Secret's data field
      # required: true           # Optional: Defaults to false
    ```

3. Object Reference (`objectRef`) 
    Values can be pulled from any field within any Kubernetes object using JSONPath notation.

    ```yaml
    templatingValueFrom:
    objectRef:
      apiVersion: apps/v1
      kind: Deployment
      name: my-deployment
      namespace: my-namespace  # Optional: Defaults to OperandConfig namespace
      path: .status.replicas   # JSONPath expression to the desired field
    # required: true           # Optional: Defaults to false
    ```

4. Default Value (`default`)
    The `default` field can specify a static value or reference other objects similar to the main templating system. This serves as a fallback when the primary references are not available:
    ```yaml
    templatingValueFrom:
      default:
        required: true
        configMapKeyRef:
          name: defaults
          key: standard-storage
      configMapKeyRef:
        name: global-defaults
        key: standard-storage
      secretKeyRef:
        name: backup-config
        key: storage-value
      objectRef:
        apiVersion: v1
        kind: PersistentVolume
        name: reference-pv
        path: .spec.capacity.storage
    ```
    In this example, if none of the primary value sources (configMapKeyRef, secretKeyRef, objectRef) in the main templatingValueFrom provide a value, ODLM will try to resolve values from the default section. It will first check the static defaultValue, then try the Object, Secret, and ConfigMap references in order until it finds a value. This provides multiple layers of fallbacks for your configuration.

#### Array Support in Templating

ODLM's templating system also supports complex array values, allowing you to define lists of mixed content types. Arrays are particularly useful in conditional branches (`then`/`else`) where you may need to return multiple values or complex structures based on conditions.

Arrays can contain:

*   **Literal values** (`literal`) - Simple static values (strings, numbers, booleans) directly included.
*   **Map structures** (`map`) - Key-value pairs defining complex objects.
*   **Reference types** -  Direct references using `configMapKeyRef`, `secretKeyRef`, or `objectRef`

**Examples:**

```yaml
templatingValueFrom:
  conditional:
    expression:
      equal:
        left:
          objectRef:
            apiVersion: v1
            kind: ConfigMap
            name: environment-config
            path: data.environment
        right:
          literal: "production"
    then:
      array:
        - literal: "value1"              # Simple string value
        - map:                           # Map structure with static values
            hostname: "backend-service"
            enabled: true
            port: 8080
        - map:                           # Map with dynamic reference
            apikey: 
              secretKeyRef:
                name: api-secrets
                namespace: default
                key: api-key
        - configMapKeyRef:               # Direct ConfigMap reference
            name: config-values
            namespace: default
            key: array-item-2
    else:
      literal: "default-value"
```
When the condition evaluates to true, ODLM processes this array and returns a JSON array containing all the resolved values:

```yaml
[
  "value1",
  {"hostname":"backend-service","enabled":true,"port":8080},
  {"apikey":"secret-api-key-value"},
  "config-value-from-key"
]
```
The resulting array preserves the structure of each item while resolving any reference types to their actual values

Empty values in conditional branches are properly handled - if a condition evaluates to false and no else branch is provided, the corresponding field will be removed from the final configuration rather than being set to an empty value.

#### Conditional Logic
The templating system in ODLM offers powerful conditional logic capabilities that allow dynamic configuration based on cluster state. 
This enables administrators to create flexible, environment-aware configurations without manual intervention.

1. Simple Conditional Expressions
    Basic conditional expressions evaluate a condition and provide different values based on the result:
  
    **Examples:**

    ```yaml
    templatingValueFrom:
      conditional:
        expression:
          equal:
            left:
              objectRef:
                apiVersion: v1
                kind: ConfigMap
                name: cluster-info
                namespace: default
                path: .data.environment
            right:
              literal: "production"
        then:
          literal: "50Gi"  # Value if condition is true
        else:
          literal: "10Gi"  # Value if condition is false
    ```
    This example configures storage size based on the environment value in `cluster-info` ConfigMap - using 50Gi for production environments and 10Gi for others.

2. Advanced Expression Evaluation
    Complex conditions can be created using logical operators (`and`, `or`, `not`). These operators take a list of sub-expressions.

    - **`and`**: All sub-expressions must be true.
    - **`or`**: At least one sub-expression must be true.
    - **`not`**: Negates the truth value of a sub-expression.

    **Examples:**

    ```yaml
    templatingValueFrom:
    conditional:
      expression:
        and:
          - equal:
              left:
                objectRef:
                  apiVersion: v1
                  kind: ConfigMap
                  name: cluster-info
                  path: .data.environment
              right:
                literal: "production"
          - notEqual:
              equal:
                left:
                  objectRef:
                    apiVersion: v1
                    kind: ConfigMap
                    name: cluster-info
                    path: .data.region
                right:
                  literal: "dev-region"
      then:
        literal: "3"  # replicas value for production in non-dev regions
      else:
        literal: "1"  # replicas value for other environments
    ```
    This example sets the number of replicas to 3 only if the environment is "production" AND the region is not "dev-region". Otherwise, it defaults to 1 replica.

3. Comparison Operations
    The templating system supports multiple comparison operators: 
    *   **`equal`**: Checks if `left` == `right`.
    *   **`notEqual`**: Checks if `left` != `right`.
    *   **`greaterThan`**: Checks if `left` > `right`.
    *   **`lessThan`**: Checks if `left` <> `right`.

    The `left` and `right` operands can be static `literal` values or dynamic references (`configMapKeyRef`, `secretKeyRef`, `objectRef`).

    **Examples:**

    ```yaml
    templatingValueFrom:
      conditional:
        expression:
          greaterThan:
            left:
              objectRef:
                apiVersion: v1
                kind: PersistentVolumeClaim
                name: data-pvc
                path: .spec.resources.requests.storage
            right:
              literal: "10Gi"
        then:
          literal: "LargeVolumePolicy"
        else:
          literal: "StandardVolumePolicy"
    ```
    This example checks if the storage request of a PersistentVolumeClaim is greater than 10Gi and sets a specific volume policy accordingly.

4. Kubernetes resource quantities
    The system intelligently handles resource quantity comparisons, automatically parsing Kubernetes resource notation:
    *   **CPU**: `m` (millicores), e.g., `500m`, `1` (whole core)
    *   **Memory**: `Ki`, `Mi`, `Gi`, `Ti`, `Pi`, `Ei` (kibibytes, mebibytes, gibibytes, etc.), e.g., `256Mi`, `4Gi`
    *   **Storage**: Same units as Memory, often used for PersistentVolumes, e.g., `10Gi`, `1Ti`
    This allows for direct comparison without manual conversion.

    **Examples:**

    ```yaml
    templatingValueFrom:
    conditional:
      expression:
        greaterThan:
          left:
            objectRef:
              apiVersion: v1
              kind: Node
              name: worker-1
              path: .status.allocatable.memory
          right:
            literal: "16Gi"
      then:
        literal: "8Gi"  # Higher memory limit for nodes with >16Gi
      else:
        literal: "4Gi"  # Lower memory limit for smaller nodes
    ```
    This example checks if the allocatable memory of a node is greater than 16Gi and sets a higher memory limit accordingly.

5. Numeric and String Comparisons
    The comparison operators perform type-aware comparisons based on the resolved values of the `left` and `right` operands:

    *   **Numeric Comparison:** If both values can be interpreted as numbers, a numeric comparison is performed. String representations of numbers are converted.
        - 5 > 3 evaluates to true.
        - "5" > "3" evaluates to true (strings are converted to numbers).
        - "10Gi" > "5Gi" evaluates to true (Kubernetes quantities are compared numerically).
        - "500m" < "1" evaluates to true (CPU quantities are compared numerically).
    *   **String Comparison:** If values cannot be interpreted as numbers, a lexicographical (alphabetical) string comparison is performed.
        - "abc" < "xyz" evaluates to true.
        - "apple" > "banana" evaluates to false.

### How does Operator create the individual operator CR

Jenkins Operator has one CRD: Jenkins:

The OperandConfig CR has

```yaml
- name: jenkins
  spec:
    jenkins:
      service:
        port: 8081
```

The Jenkins Operator CSV has

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

## OperandRequest Spec

OperandRequest defines which operator/operand you want to install in the cluster.

**NOTE:** OperandRequest custom resource is used to trigger a deployment for Operators and Operands.

### OperandRequest sample to create custom resource via OperandConfig

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
4. (optional) `registryNamespace` identifies the namespace in which the OperandRegistry CR is defined. **Note:** If the `registryNamespace` is not specified then it is assumed that the OperandRegistry CR is in the current (OperandRequest's) namespace.
5. `operands` in the CR is a list of operands.
6. `name` of operands in the CR must match a name specification in an OperandRegistry's CR.
7. (optional) The `bindings` of the operands is a map to get and rename the secret and/or configmap from the provider and create them in the requester's namespace. If the requester wants to rename the secret and/or configmap, they need to know the key of the binding in the OperandBindInfo. If the key of the bindings map is prefixed with public, it means the secret and/or configmap can be shared with the requester in the other namespace. If the key of the bindings map is prefixed with private, it means the secret and/or configmap can only be shared within its own namespace.
8. (optional) `secret` names a secret that should be created in the requester's namespace with formatted data that can be used to interact with the service.
9. (optional) `configmap` names a configmap that should be created in the requester's namespace with formatted data that can be used to interact with the service.

### OperandRequest sample to create custom resource via OperandRequest

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: OperandRequest
metadata:
  name: example-service
  namespace: example-service-ns
spec:
  requests:
  - registry: example-service
    registryNamespace: example-service-ns
    operands:
    - name: jenkins
      kind: Jenkins [1]
      apiVersion: "jenkins.io/v1alpha2" [2]
      instanceName: "example" [3]
      spec: [4]
        service:
          port: 8081
```

1. `kind` of the target custom resource. If it is set in the operand item, ODLM will create CR according to OperandRequest only and get rid of the alm-example and OperandConfig.
2. `apiVersion` of the target custom resource.
3. `instanceName` is the name of the custom resource. If `instanceName` is not set, the name of the custom resource will be created with the name of the OperandRequest as a prefix.
4. `spec` is the spec field of the target CR.

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
  registryNamespace: example-service-ns [5]
  description: "Binding information that should be accessible to jenkins adopters" [6]
  bindings: [7]
    public:
      secret: jenkins-operator-credentials-example [8]
      configmap: jenkins-operator-base-configuration-example [9]
```

Fields in this CR are described below.

1. `name` of the OperandBindInfo
2. `namespace` of the OperandBindInfo
3. The `operand` should be the the individual operator name.
4. The `registry` identifies the name of the OperandRegistry CR in which this operand information is registered.
5. (optional) `registryNamespace` identifies the namespace in which the OperandRegistry CR is defined. **Note:** If the `registryNamespace` is not specified then it is assumed that the OperandRegistry CR is in the current (OperandBindInfo's) namespace.
6. `description` is used to add a detailed description of a service.
7. The `bindings` section is used to specify information about the access/configuration data that is to be shared. If the key of the bindings map is prefixed with public, it means the secret and/or configmap can be shared with the requester in the other namespace. If the key of the bindings map is prefixed with private, it means the secret and/or configmap can only be shared within its own namespace. If the key of the bindings map is prefixed with protected, it means the secret and/or configmap can only be shared if it is explicitly declared in the OperandRequest.
8. The `secret` field names an existing secret, if any, that has been created and holds information that is to be shared with the requester.
9. The `configmap` field identifies a configmap object, if any, that should be shared with the requester

ODLM will use the OperandBindInfo CR to pass information to an adopter when they create a OperandRequest to access the service, assuming that both have compatible scopes. ODLM will copy the information from the shared service's "OperandBindInfo.bindinfo[].secret" and/or "OperandBindInfo.bindinfo[].configmap" to the requester namespace.

**NOTE:** If in the OperandRequest, there is no secret and/or configmap name specified in the bindings or no bindings field in the element of operands, ODLM will copy the secret and/or configmap to the requester's namespace and rename them to the name of the OperandBindInfo + secret/configmap name.

## E2E Use Case

1. User installs ODLM from OLM

    The ODLM will automatically generate two default CRD CRs, since OperandRegistry and OperandConfigs CRs don't define the state, so it should be fine.

2. **Optional:** Users can update the OperandConfigs CR with their own parameter values

    Users can skip this step if the default configurations match the requirement.

3. User creates the OperandRequest CR from OLM

    This tells ODLM that users want to install some of the individual operator/operand.

4. The rest works will be done by OLM and ODLM

    Finally, users will get what they want.

## Operator/Operand Upgrade

- For operator/operand upgrade, you only need to publish your operator OLM to your operator channel, and OLM will handle the upgrade automatically.
- If there are major version, then you may want to update `channel` in `OperandRegistry` to trigger upgrade.
