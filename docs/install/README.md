## Installing Knative with GCP

To install Knative with GCP from master,

### Using [ko](http://github.com/google/ko)

```shell
ko apply -f ./config
```

Applying the `config` dir will:

- create a new namespace, `cloud-run-events`.
- install new CRDs,
  - `pullsubscriptions.pubsub.cloud.run`
- install a controller to operate on the new resources,
  `cloud-run-events/cloud-run-events-controller`.
- install a new service account, `cloud-run-events/cloud-run-events-controller`.
- provide RBAC for that service account.

## Uninstalling Knative with GCP

### Using [ko](http://github.com/google/ko)

```shell
ko delete -f ./config
```
