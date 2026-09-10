# Working Preferences & Standards

## How I use AI

- **Bug fixes** — the most common use case. Understand the problem before suggesting a fix.
- **Understanding complex areas** — especially the OperandConfig templating system and CatalogSource selection logic.
- **Feature updates** — occasional; help think through implications for existing consumers before implementing.

## Communication preferences

- Be direct and technical. No filler phrases ("Great!", "Certainly!", etc.).
- Investigate before answering — never speculate about code you haven't read.
- When explaining unfamiliar code: start with what it *does*, then how it works. Top-down is easier than bottom-up.
- Flag assumptions and caveats explicitly, especially around the templating system or CatalogSource logic.

## Code style & engineering standards

- **Minimal changes.** Produce the smallest diff that solves the problem. No opportunistic refactors.
- **Trace every changed line** back to the stated requirement.
- **Go conventions.** Follow standard Go idioms and the existing style in the file being edited.

## What to be careful about

- **OperandConfig templating is complex.** Read the controller logic before modifying anything in this area — it is easy to break silently.
- **CatalogSource selection is opaque.** When debugging operator install or subscription failures, this logic is a likely culprit. Don't assume the selection is straightforward.
- **Both CPfs and standalone Cloud Pak consumers exist.** Changes to ODLM behaviour may affect Cloud Paks that use it independently of CPfs, not just CPfs itself.
