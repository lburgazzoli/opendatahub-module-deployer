# OpenDataHub module deployer

`opendatahub-module-deployer` loads a local `PlatformModuleConfig`, renders its
Helm chart, and applies the resulting Kubernetes objects. The module CR is an
input identity only: this library does **not** create or update it.

Import the library as `github.com/lburgazzoli/opendatahub-module-deployer/pkg/deployer`.

The preferred API uses option structs:

```go
cfg, err := deployer.LoadConfigFile("modules/example.yaml")
if err != nil {
    return err
}

d, err := deployer.For(cfg, deployer.Options{
    ChartsPath: "/opt/odh/charts",
    Namespace:  "opendatahub",
    Resolvers: map[string]deployer.ResolverFunc{
        "gatewayconfig": gatewayStatus,
    },
    Labels: map[string]string{"app.kubernetes.io/managed-by": "my-operator"},
})
if err != nil {
    return err
}

objects, err := d.Run(ctx, kubernetesClient, deployer.RunOptions{
    Owner:  moduleOwner,
    Values: map[string]any{"featureFlag": true},
})
```

`For` resolves the local chart path, creates the Helm engine and deploy action,
and reads `relatedImages` environment variables once. Images and `spec.config`
become Helm source defaults; each `Run` supplies fresh service values and can
merge additional values through `RunOptions.Values`. The engine loads the chart
when needed and caches it; the chart directory is assumed stable. `Run` returns
all deployed objects on success, including unchanged objects. An apply error
may leave earlier objects deployed; there is no rollback.

Functional helpers also work, for example `deployer.Resolver(...)`,
`deployer.WithLabels(...)`, and `deployer.RunWithValues(...)`.

The current utilities-v2 deploy action requires `Owner`. The deployer no longer
has a separate ownerless apply path; nil-owner support must be added to that
action before ownerless runs can succeed.

See [the YAML reference](docs/module-config.md), [architecture](docs/architecture.md),
and [development guide](docs/development.md).

For now, builds require the adjacent `../odh-platform-utilities/v2` checkout:
the fork's published `utilities-v2` branch does not yet contain that module.
The relative replacement in `go.mod` can become a pinned remote replacement
after the branch is published.
