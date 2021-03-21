<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Operand Deployment Lifecycle Manager (ODLM) and Operator Lifecycle Manager (OLM)](#operand-deployment-lifecycle-manager-odlm-and-operator-lifecycle-manager-olm)
  - [What is Operator Lifecycle Manager?](#what-is-operator-lifecycle-manager)
  - [What are operands?](#what-are-operands)
  - [How does the ODLM work?](#how-does-the-odlm-work)
  - [Application Lifecycle Management](#application-lifecycle-management)
    - [installation](#installation)
    - [uninstall](#uninstall)
  - [Dependency Management](#dependency-management)
  - [Additional Features](#additional-features)
    - [Binding Information sharing](#binding-information-sharing)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Operand Deployment Lifecycle Manager (ODLM) and Operator Lifecycle Manager (OLM)

When it comes to deploying operator services, users will always want to know what is the difference between Operand Deployment Lifecycle Manager (ODLM) and Operator Lifecycle Manager (OLM). In this document, we compare them and discuss what advantages of using ODLM and when you should pick ODLM as an extension of OLM.

## What is Operator Lifecycle Manager?

Operator Lifecycle Manager (OLM) helps users install, update, and manage the lifecycle of Kubernetes native applications (Operators) and their associated services running across their OpenShift Container Platform clusters.
It implements features, like application lifecycle management and dependency management. For more details, you can check [operator-lifecycle-manager](https://github.com/operator-framework/operator-lifecycle-manager)

## What are operands?

Operator is a method of packaging, deploying and managing a Kubernetes application.
Operands are the services and applications that Operator manage.

For example, cert-manager operator deploys a cert-manager deployment, then the cert-manager-operator is an operator and the cert-manager deployment is an operand.

## How does the ODLM work?

The ODLM manages the deployment of the operands for OLM managed operators.  This provides a mechanism for dynamically deploying dependent (and optionally, shared) services in a prescriptive manner and allowing these deployments to interact when and where needed in a scoped fashion.

## Application Lifecycle Management

### installation

- **OLM** can be used to deploy a single operator by creating a Subscription.
- **ODLM** can be used to manage the lifecycle of a group of operands, compared with operator lifecycle manager, **ODLM** focuses on the management of both operands and operators. Users can create an OperandRequest to install a group of Operators and specific CRs for these operators.

### uninstall

- When users use **OLM** only, they need to delete every created custom resource and all the operators.
- When using **ODLM**, if users don't need an operator application anymore, they can delete the OperandRequest created for the application. **ODLM** will handle the logic of checking if there are other users requesting this application and delete both custom resources and operators when they are no longer referenced.

## Dependency Management

- **OLM** creates all required, dependent operators automatically by creating additional Subscriptions automatically.  Each operator statically defines all of it's REQUIRED dependencies in the form of CustomResourceDefinitions, API Service Definitions or Operator Package versions.  
  - **OLM** operators declare which CRDs, and API Services are PROVIDED.
  - **OLM** operators declare which CRDs, API Services and other Operator Package Versions are REQUIRED.
  - All dependencies are statically defined as REQUIRED (vs. optional, or preferred), which result in all dependencies must be installed as a unit.
  - All dependencies must be in the same OperatorGroup, which may require the operators to be in the same namespace. 
  - Example:  The _Parent_ Operator REQUIRES the _Child_ CRD and the _Child_ Operator PROVIDES the _Child_ CRD.  When the _Parent_ Operator Subscription is created, the _Child_ Subscription is dynamically created, satisfying the _Child_ CRD dependency.
- **ODLM** manages dependencies by creating _OperandRequests_.  Instead of providing a statically REQUIRED dependency, the Operator's creates a soft dependency using one or more OperandRequests for the dependent Operators as needed.  
  - **ODLM** *OperandRequests* decouple the lifecycle of the dependant Operators and allows dependencies to be installed (Subscribed) on demand.
    - The dependent _OperandRequests_ can be statically defined in the `alm-examples` section of the OLM _ClusterServiceVersion_, or can be created dynamically by the Operator's controller.
  -  OLM Operators, managed by **ODLM** can be managed in any namespace combination allowing single-namespace installations to be shared between namespaces, avoiding the need for All-Namespace or Multi-Namespace OperatorGroups and Install Modes.
  -  Example: The _Parent_ Operator controller REQUIRES the _Child_ Operator, by expressing the dependency in the Controller code, creating an OperandRequest.  This dependency is NOT defined in the OLM ClusterServiceVersion.

## Additional Features

### Binding Information sharing

**ODLM** can use the OperandBindInfo to claim the information that services want to share with the requester. The ODLM will use the request to copy the secret and/or configmap to the namespace of the OperandRequest. For more details, you can check [OperandBindInfo](./operand-deployment-lifecycle-manager.md#operandbindinfo-spec)
