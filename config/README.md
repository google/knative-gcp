# Welcome to the knative-gcp config directory!

The files in this directory are organized as follows:

- `core/`: the elements that are required for knative-gcp to function,
- `monitoring/`: an installable bundle of tooling for assorted observability
  functions,
- `istio/`: the istio configuration,
- `*.yaml`: symlinks that form a particular "rendered view" of the
  knative-gcp configuration.

## Core

The core is complex enough that it further breaks down as follows:

- `roles/`: The (cluster) roles needed for the core controllers to function, or
  to plug knative-gcp into standard Kubernetes RBAC constructs.
- `configmaps/`: The configmaps that are used to configure the core components.
- `resources/`: The knative-gcp resource definitions.
- `webhooks/`: The knative-gcp mutating and validating admission webhook
  configurations, and supporting resources.
- `deployments/`: The knative-gcp executable components and associated
  configuration resources.
