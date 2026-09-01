# Project Overview

## What this project is

`operand-deployment-lifecycle-manager` (ODLM) is a meta operator that extends OLM's framework to install other OLM-managed operators and to install/configure operand resources for those operators, as defined in the `OperandConfig` CR.

In the CPfs context, `ibm-common-service-operator` creates an `OperandRegistry` and `OperandConfig` tailored for all CPfs operators, making ODLM the backbone of CPfs lifecycle management.

## Main goals

1. **Operator lifecycle management** — install and manage OLM operators beyond what OLM itself provides.
2. **Operand configuration** — apply and reconcile operand resources for installed operators via `OperandConfig`.
3. **Serve CPfs** — primary consumer is `ibm-common-service-operator`, which drives ODLM to manage the full CPfs operator suite.

## Key stakeholders / users

- **`ibm-common-service-operator`** — primary consumer; drives ODLM with CPfs-specific `OperandRegistry` and `OperandConfig`.
- **IBM Cloud Pak teams** — some Cloud Paks use ODLM standalone with their own `OperandRegistry`/`OperandConfig` for non-CPfs operators.
- **CPfs platform team** — maintainers.

## Key CRDs

| CRD | Purpose |
|-----|---------|
| `OperandRegistry` | Declares which operators are available and where to find them (CatalogSource, channel, etc.) |
| `OperandConfig` | Defines the desired configuration (operand resources) for each registered operator |

## Tricky areas

### OperandConfig templating system
`OperandConfig` supports a templating mechanism for operand resource values. This is complex and is a frequent source of bugs. Always read the relevant controller logic before making changes here.

### CatalogSource selection
The logic for choosing which `CatalogSource` to use when creating OLM `Subscriptions` is opaque. It involves multiple heuristics and is not straightforward to reason about from the CRD alone. When debugging subscription or install failures, this is a key area to investigate.

## Key workflows

- **Bug fixes** — most common work.
- **Feature updates** — occasional, usually driven by CPfs or Cloud Pak requirements.
