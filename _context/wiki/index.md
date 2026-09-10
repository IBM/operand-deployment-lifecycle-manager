# operand-deployment-lifecycle-manager Wiki

Meta operator (ODLM) that extends OLM's framework to install and configure other OLM-managed operators and their operands. Installed by `ibm-common-service-operator` as the backbone of CPfs operator lifecycle management.

## Contents

| File | What's in it |
|------|-------------|
| [project.md](project.md) | Project overview, goals, key CRDs, tricky areas |
| [preferences.md](preferences.md) | Working standards, coding style, AI collaboration preferences |

## Quick orientation

- ODLM extends OLM to install other operators and configure their operands via `OperandRegistry` and `OperandConfig` CRDs.
- `ibm-common-service-operator` creates the `OperandRegistry` and `OperandConfig` tailored for CPfs operators.
- Some Cloud Paks have used ODLM standalone with their own `OperandRegistry`/`OperandConfig` for non-CPfs operators.
- Tricky areas: the **OperandConfig templating system** and the **opaque CatalogSource selection logic** for Subscriptions.
