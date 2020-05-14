# CloudPubSubSource Example

## Overview

This sample shows how to configure `CloudPubSubSources`. The `CloudPubSubSource`
fires a new event each time a message is published on a
[Cloud Pub/Sub topic](https://cloud.google.com/pubsub/). This source sends
events using a Push-compatible format.

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Pub/Sub enabled Service Account](../../install/pubsub-service-account.md)

## Deployment

1. Create a GCP PubSub Topic. If you change its name (`testing`), you also need
   to update the `topic` in the [`CloudPubSubSource`](cloudpubsubsource.yaml)
   file.

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `googleServiceAccount` with the Pub/Sub enabled service account you
      created in
      [Create a Pub/Sub enabled Service Account](../../install/pubsub-service-account.md).

   1. If you are using standard Kubernetes secrets, but want to use a
      non-default one, update `secret` with your own secret.

   ```shell
   gcloud pubsub topics create testing
   ```

1. Create a [`CloudPubSubSource`](cloudpubsubsource.yaml)

   ```shell
   kubectl apply --filename cloudpubsubsource.yaml
   ```

1. Create a [`Service`](event-display.yaml) that the CloudPubSubSource will sink
   into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

## Publish

Publish messages to your GCP PubSub topic:

```shell
gcloud pubsub topics publish testing --message='{"Hello": "world"}'
```

## Verify

We will verify that the published event was sent by looking at the logs of the
service that this CloudPubSubSource sinks to.

1. We need to wait for the downstream pods to get started and receive our event,
   wait 60 seconds. You can check the status of the downstream pods with:

   ```shell
   kubectl get pods --selector app=event-display
   ```

   You should see at least one.

1. Inspect the logs of the service:

   ```shell
   kubectl logs --selector app=event-display -c user-container --tail=200
   ```

You should see log lines similar to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: com.google.cloud.pubsub.topic.publish
  source: //pubsub.googleapis.com/projects/PROJECT_ID/topics/TOPIC_NAME
  id: 951049449503068
  time: 2020-01-24T18:29:36.874Z
  datacontenttype: application/json
Extensions,
  knativearrivaltime: 2020-01-24T18:29:37.212883996Z
  knativecemode: push
  traceparent: 00-7e7fb503ae694cc0f1cbf84ea63354be-f8c4848c9c11e073-00
Data,
  {
    "subscription": "cre-pull-7b35a745-877f-4f1f-9434-74062631a958",
    "message": {
      "messageId": "951049449503068",
      "data": "eyJIZWxsbyI6ICJ3b3JsZCJ9Cg==",
      "publishTime": "2020-01-24T18:29:36.874Z"
    }
  }
```

## What's Next

1. For more details on Cloud Pub/Sub formats refer to the
   [Subscriber overview guide](https://cloud.google.com/pubsub/docs/subscriber).
1. For integrating with Cloud Storage see the
   [Storage example](../../examples/cloudstoragesource/README.md).
1. For integrating with Cloud Scheduler see the
   [Scheduler example](../../examples/cloudschedulersource/README.md).
1. For integrating with Cloud Audit Logs see the
   [Cloud Audit Logs example](../../examples/cloudauditlogssource/README.md).
1. For integrating with Cloud Build see the
   [Build example](../../examples/cloudbuildsource/README.md).
1. For more information about CloudEvents, see the
   [HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## Cleaning Up

1. Delete the `CloudPubSubSource`

   ```shell
   kubectl delete -f ./cloudpubsubsource.yaml
   ```

1. Delete the `Service`

   ```shell
   kubectl delete -f ./event-display.yaml
   ```
