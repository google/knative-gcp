## Installing Knative with GCP

To install Knative with GCP from master,

### Using [ko](http://github.com/google/ko)

```shell
ko apply -f ./config
```

Applying the `config` dir will:

- create a new namespace, `cloud-run-events`.
- install new CRDs,
  - `channel.messaging.cloud.google.com`
  - `decorator.messaging.cloud.google.com`
  - `storages.events.cloud.google.com`
  - `pullsubscriptions.pubsub.cloud.google.com`
  - `topics.pubsub.cloud.google.com`
- install a controller to operate on the new resources,
  `cloud-run-events/cloud-run-events-controller`.
- install a new service account, `cloud-run-events/cloud-run-events-controller`.
- provide RBAC for that service account.

## Uninstalling Knative with GCP

### Using [ko](http://github.com/google/ko)

```shell
ko delete -f ./config
```

### Using [releases](https://github.com/google/knative-gcp/releases)

```shell
kubectl apply --filename https://github.com/google/knative-gcp/releases/download/v0.9.0/cloud-run-events.yaml
```

Applying the `config` dir will:

- create a new namespace, `cloud-run-events`.
- install new CRDs,
  - `channel.messaging.cloud.google.com`
  - `decorator.messaging.cloud.google.com`
  - `storages.events.cloud.google.com`
  - `pullsubscriptions.pubsub.cloud.google.com`
  - `topics.pubsub.cloud.google.com`
- install a controller to operate on the new resources,
  `cloud-run-events/cloud-run-events-controller`.
- install a new service account, `cloud-run-events/cloud-run-events-controller`.
- provide RBAC for that service account.

## Uninstalling Knative with GCP

### Using [ko](http://github.com/google/ko)

```shell
kubectl delete --filename https://github.com/google/knative-gcp/releases/download/v0.9.0/cloud-run-events.yaml
```
