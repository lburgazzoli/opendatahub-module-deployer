# Development

Requires Go 1.26.8 or newer. The module uses the local `utilities-v2` checkout
of `lburgazzoli/opendatahub-platform-utilities` through a relative Go module
replacement. It expects that checkout at `../odh-platform-utilities/v2`.
The published fork branch currently lacks the v2 module; once it is pushed,
replace the local path with a pinned remote version. Chart rendering uses a
pinned `renderer-helm` main revision.

The public Go package is `pkg/deployer`; import it as
`github.com/lburgazzoli/opendatahub-module-deployer/pkg/deployer`.

Use the Makefile as the project interface:

```sh
make help
make deps
make fmt
make test
make lint
make vulncheck
```

Unit tests use local Helm chart fixtures and a fake Kubernetes client. They do
not require a cluster. The default test target enables the race detector and
disables test-result caching. End-to-end tests, if added later, use the `e2e`
build tag and `make test/e2e`.

The ownerless Run regression tests currently fail because the local
utilities-v2 deploy action rejects a nil owner. They remain in place until the
action supports ownerless deployment.

The relative replacement is temporary and intentionally explicit. It keeps
this repository buildable against the adjacent fork checkout without claiming
that an unpublished remote revision exists.
