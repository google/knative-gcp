## Installing Cloud Run Events

Installing Cloud Run Events from master.

### Using [ko](http://github.com/google/ko)

```shell
ko apply -f ./config
```

Applying the `config` dir will:

- Create a new namespace, `cloud-run-events`.
- Install new CRDs:
  - `channels.pubsub.cloud.run`
  - `pullsubscriptions.pubsub.cloud.run`
  - `topics.pubsub.cloud.run`
- Install a controller Deployment to operate on the new resources,
  `cloud-run-events/cloud-run-events-controller`.
- Install a webhook Deployment to validate and mutate the new resources, `cloud-run-events/cloud-run-events-webhook`.
- Install a new ServiceAccount, `cloud-run-events/cloud-run-events-controller`.
- Provide RBAC for that ServiceAccount.

## Uninstalling Cloud Run Events

### Using [ko](http://github.com/google/ko)

```shell
ko delete -f ./config
```
