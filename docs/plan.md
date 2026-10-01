# Implementation plan

The initial implementation is organized around local config loading, Helm
engine and deploy-action initialization plus image capture in `For`, per-run
values resolution and metadata post-rendering, and one deploy-action path.
Public API and behavior are described in the README and architecture document.

The compatibility boundary is intentionally narrow: local chart directories,
one module per deployer instance, and caller-provided service resolvers.
Runlevel remains a scheduling hint for the caller. Module CR management,
cross-module ordering, garbage collection, and rollback are outside this
library's responsibility.

The Helm renderer revision is pinned in `go.mod`. The local utilities-v2
checkout is used through a temporary sibling-directory replacement because
the fork's published branch does not yet contain the v2 module. After that
commit is published, replace the local path with its remote pseudo-version.
Deploy-action support for nil owners is still needed to make ownerless runs
succeed. Future work can add chart source types without changing the YAML format
or preferred struct-option API.
