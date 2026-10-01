# PlatformModuleConfig YAML

This is a **local file**, shaped like a Kubernetes resource but not registered
as a CRD. One file describes one module. See [the full example](../examples/example-module.yaml).

| Field | Meaning |
| --- | --- |
| `apiVersion`, `kind` | Must be `deployer.opendatahub.io/v1alpha1`, `PlatformModuleConfig`. |
| `metadata.name` | Module name; also the default chart path and Helm release name. |
| `spec.moduleRef` | API version, kind, and name of the module CR managed by the caller. The deployer filters chart output of this GVK. |
| `spec.chart.name` | Optional Helm release name; defaults to `metadata.name`. |
| `spec.chart.path` | Optional local chart directory relative to `Options.ChartsPath`; defaults to `metadata.name`. Absolute and escaping paths are rejected. |
| `spec.chart.version` | Optional `ReleaseVersion` passed to the Helm renderer. For a local chart directory it does not select another directory or check `Chart.yaml`'s version. |
| `spec.runlevel` | Non-negative ordering hint exposed to the caller; it does not gate `Run`. |
| `spec.relatedImages` | Environment variable names. Their non-empty values are captured at `For`. |
| `spec.config` | Arbitrary nested YAML map passed to `.Values.config`. |
| `spec.services` | Service names; each must have a resolver registered in `Options.Resolvers`. |

For example, with `RELATED_IMAGE_FOO=quay.io/example/foo:1`, the chart receives
values equivalent to:

```yaml
images:
  RELATED_IMAGE_FOO: quay.io/example/foo:1
config:
  key: val
services:
  gatewayconfig:
    ready: true
```

`RunOptions.Values` can provide additional Helm values or override source
defaults such as `config`; resolver-produced service fields take precedence
over overlapping `services` values. The resolver supplies the `gatewayconfig`
map at each `Run`. It should remove status fields the chart does not need. For
example:

```go
func gatewayStatus(ctx context.Context, cli client.Client) (map[string]any, error) {
    gateway := &unstructured.Unstructured{}
    gateway.SetAPIVersion("services.platform.opendatahub.io/v1alpha1")
    gateway.SetKind("GatewayConfig")
    if err := cli.Get(ctx, client.ObjectKey{Name: "cluster"}, gateway); err != nil {
        return nil, err
    }
    status, _, err := unstructured.NestedMap(gateway.Object, "status")
    if err != nil {
        return nil, err
    }
    delete(status, "conditions")
    delete(status, "observedGeneration")
    delete(status, "phase")
    return status, nil
}
```

The `cluster` name in the generic example is illustrative. Current Ray and
Spark CRDs in the adjacent orchestrator restrict their CR names to other
values; this library does not change those CRDs.
