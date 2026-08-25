# Review

- Root cause: all three configured names exist as repository Secrets, while the workflow read repository Variables, so the build received empty strings and correctly fell back to local search.
- Fix: read those same names from `secrets.*`, still guarded by `environment == 'development'`.
- The Search-only key remains public in the generated static frontend by Algolia design; an Admin key remains forbidden.
- RED confirmed the old Variables wiring; GREEN: 6 workflow tests and YAML parsing passed.
- No application, Production, routing, or documentation content behavior changed.
