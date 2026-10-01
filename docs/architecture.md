# Architecture

The deployer has two phases:

1. `For(cfg, Options{...})` validates the local config and resolver coverage,
   resolves a chart directory beneath `ChartsPath`, initializes one Helm
   engine and one deploy action, and reads the listed image environment
   variables through Viper. The Helm source receives `images` and `config` as
   fixed default values. Chart loading and caching are internal to the Helm
   engine; the chart directory is assumed stable.
2. `Run(ctx, cli, RunOptions{...})` calls each service resolver with the supplied
   Kubernetes client, merges additional `RunOptions.Values` with fresh
   `services` as render-time Helm values, filters module-CR output, applies
   per-run labels and annotations through a post-renderer, then deploys the
   other objects in dependency order.

The renderer is `k8s-manifest-kit/renderer-helm`; the deployment and resource
helpers come from the local `utilities-v2` checkout of the user's
platform-utilities fork. Constructor and Run options use its generic `ApplyTo`
pattern. Structs are the documented primary style; functional options compose
with them. Go code is in `pkg/deployer`.

The engine's per-run post-renderer merges constructor and Run labels and
annotations into all rendered objects. The reusable utilities-v2 deploy action
then orders and applies every resource. With an owner, it adds owner metadata
and controller references where permitted. The owner is assumed to be a
cluster-scoped singleton module CR; no ClusterRole or ClusterRoleBinding
exclusions are configured. CRDs retain the deploy action's built-in ownership
exception. Without an owner, the deployer still calls the action, but the
current utilities-v2 checkout rejects the run. Supporting ownerless deployment
requires a change in that action. The default field manager is
`opendatahub-module-deployer`, unless overridden through options.

The module CR identified by `spec.moduleRef` is never read or applied. The Helm
renderer filters out any object with that GVK. The caller is responsible for
creating the module CR and may separately provide any existing Kubernetes
object as the deployment owner.

`Run` returns the full prepared desired-object set on success, not a live-state
snapshot. There is no garbage-collection or rollback phase. A failed apply may
leave earlier objects changed.
