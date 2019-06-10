## Installing Cloud Run Events

To install Cloud Run Events from master,

### Using [ko](http://github.com/google/ko)

```shell
ko apply -f ./config
```

Applying the `config` dir will:

- create a new namespace, `cloud-run-events`.
- install new CRDs,
  - `PubSubSource.events.cloud.run`
- install a controller to operate on the new resources,
  `cloud-run-events/cloud-run-events-controller`.
- install a new service account, `cloud-run-events/cloud-run-events-controller`.
- provide RBAC for that service account.

## Uninstalling Cloud Run Events

### Using [ko](http://github.com/google/ko)

```shell
ko delete -f ./config
```
