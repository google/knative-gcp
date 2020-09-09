# Cloud Pub/Sub Channel Example

This sample shows how to configure a Channel backed by Cloud Pub/Sub. This is an
implementation of a
[Knative Eventing Channel](https://github.com/knative/eventing/blob/master/docs/spec/channel.md)
intended to provide a durable messaging solution.

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md). Remember to
   install [Eventing](https://knative.dev/docs/eventing/) as part of the
   installation procedure.

1. [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md)

## Deployment

1. Create the `Channel` in [channel.yaml](channel.yaml).

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `serviceAccountName` with the Kubernetes service account you
      created in
      [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md),
      which is bound to the Pub/Sub enabled Google service account.

   1. If you are using standard Kubernetes secrets, but want to use a
      non-default one, update `secret` with your own secret.

   ```shell
   kubectl apply --filename channel.yaml
   ```

   After a moment, the demo channel should become ready.

   ```shell
   kubectl get channels.messaging.cloud.google.com demo
   ```

1. Create a subscriber from [event-display.yaml](event-display.yaml).

   ```shell
   kubectl apply --filename event-display.yaml
   ```

1. Create a [`Subscription`](subscription.yaml).

   ```shell
   kubectl apply --filename subscription.yaml
   ```

   After a moment, the subscription will become ready.

   ```shell
   kubectl get subscription demo
   ```

1. Create an Event Source, in this case, a `PingSource` from
   [source.yaml](source.yaml).

   ```shell
   kubectl apply --filename source.yaml
   ```

   This will send an event through the `demo` channel every minute on the
   minute.

## Verify

This results in the following:

```
[hello-world] --> [demo channel] -> [event-display]
```

1. Inspect the logs of the `event-display` pod:

   ```shell
   kubectl logs --selector app=event-display -c user-container
   ```

You should see log lines similar to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: dev.knative.sources.ping
  source: /apis/v1/namespaces/default/pingsources/hello-world
  id: 37a8a186-acc0-4c63-b1ad-a8dac9caf288
  time: 2019-08-26T20:48:00.000475893Z
  datacontenttype: application/json
Data,
  {
    "hello": "world"
  }
```

These events are generated from the `hello-world` `PingSource`, sent through the
`demo` `Channel` and delivered to the `event-display` via the `demo`
`Subscription`.

## Troubleshooting

You may have issues receiving desired CloudEvent. Please use
[Authentication Mechanism Troubleshooting](../../how-to/authentication-mechanism-troubleshooting.md)
to check if it is due to an auth problem.

## What's Next

The `Channel` implements what Knative Eventing considers to be a `Channelable`.
This component can work alone, but it also works well when
[Knative Serving and Eventing](https://github.com/knative/docs) are installed in
the cluster.

## Cleaning Up

1. Delete the resources:

```shell
kubectl delete \
  --filename channel.yaml \
  --filename event-display.yaml \
  --filename subscription.yaml \
  --filename source.yaml
```
