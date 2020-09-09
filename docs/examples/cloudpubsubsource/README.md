# CloudPubSubSource Example

## Overview

This sample shows how to configure `CloudPubSubSources`. The `CloudPubSubSource`
fires a new event each time a message is published on a
[Cloud Pub/Sub topic](https://cloud.google.com/pubsub/). This source sends
events using a Push-compatible format.

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md)

## Deployment

1. Create a GCP PubSub Topic. If you change its name (`testing`), you also need
   to update the `topic` in the [`CloudPubSubSource`](cloudpubsubsource.yaml)
   file.

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `serviceAccountName` with the Kubernetes service account you
      created in
      [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md),
      which is bound to the Pub/Sub enabled Google service account.

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
  type: google.cloud.pubsub.topic.v1.messagePublished
  source: //pubsub.googleapis.com/projects/test-project/topics/testing
  id: 1314133748793931
  time: 2020-06-30T16:32:57.012Z
  dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/pubsub/v1/data.proto
  datacontenttype: application/json
Extensions,
  knativearrivaltime: 2020-06-30T16:32:58.016175839Z
  knsourcetrigger: link0.13999342310482066
  traceparent: 00-e9ce0f38d85d8333bd1a3334ead78b4d-acd063b2d3e93980-00
Data,
  {
    "subscription": "cre-src_rc3_source-for-knativegcp-test-pubsub-tr_fcdf7716-c4bd-43b9-8ccc-e6e8ff848cd4",
    "message": {
      "messageId": "1314133748793931",
      "data": "eyJIZWxsbyI6ICJ3b3JsZCJ9", // base64 encoded '{"Hello": "world"}'
      "publishTime": "2020-06-30T16:32:57.012Z"
    }
  }
```

## Troubleshooting

You may have issues receiving desired CloudEvent. Please use
[Authentication Mechanism Troubleshooting](../../how-to/authentication-mechanism-troubleshooting.md)
to check if it is due to an auth problem.

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
