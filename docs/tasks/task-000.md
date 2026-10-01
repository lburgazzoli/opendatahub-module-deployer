# Task 000 — Initial module deployer

Deliver a reusable module deployer with:

- strict local `PlatformModuleConfig` YAML loading and a documented example;
- chart and related-image initialization at `For`;
- per-run service values, metadata options, and client-supplied deployment;
- a single deploy-action path, with ownerless support pending in utilities-v2;
- protection against deploying the module CR itself;
- tests for validation, image capture, chart caching, options, resolvers, and deployment.

Acceptance checks: `make test`, `make lint`, and `make help`. The Run result
contains all prepared desired objects when deployment succeeds.
