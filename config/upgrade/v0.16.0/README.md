# Upgrade script (optional) to upgrade to v0.16.0 of knative-gcp

Starting from v0.16.0 `pubsub.cloud.google.com` API is no longer supported. This
directory contains a job that deletes the legacy
`{pullsubscription,topic}.pubsub.cloud.google.com` resources in all namespaces.

To run the upgrade script:

```shell
kubectl apply -f https://github.com/google/knative-gcp/releases/download/v0.16.0/upgrade-to-v0.16.0.yaml
```

It will create a job called v0.16.0-upgrade in the `cloud-run-events` namespace.
If you installed to a different namespace, you need to modify the upgrade.yaml
appropriately. Also the job by default runs as `controller` service account, you
can also modify that but the service account will need to have permissions to
list `Namespace`s, delete `{pullsubscription,topic}.pubsub.cloud.google.com`s.
