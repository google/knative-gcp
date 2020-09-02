# CloudSchedulerSource Example

## Overview

This sample shows how to Configure `CloudSchedulerSource` resource for receiving
scheduled events from
[Google Cloud Scheduler](https://cloud.google.com/scheduler/).

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md).

1. Create with an App Engine application in your project. Refer to this
   [guide](https://cloud.google.com/scheduler/docs/quickstart#create_a_project_with_an_app_engine_app)
   for more details. You can change the APP_ENGINE_LOCATION, but please make
   sure you also update the spec.location in
   [`CloudSchedulerSource`](cloudschedulersource.yaml)

   ```shell
   export APP_ENGINE_LOCATION=us-central
   gcloud app create --region=$APP_ENGINE_LOCATION
   ```

1. [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md)

1. Enable the `Cloud Scheduler API` on your project:

   ```shell
   gcloud services enable cloudscheduler.googleapis.com
   ```

## Deployment

1. Create a [`CloudSchedulerSource`](cloudschedulersource.yaml)

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `serviceAccountName` with the Kubernetes service account you
      created in
      [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md),
      which is bound to the Pub/Sub enabled Google service account.

   1. If you are using standard Kubernetes secrets, but want to use a
      non-default one, update `secret` with your own secret.

   ```shell
   kubectl apply --filename cloudschedulersource.yaml
   ```

1. Create a [`Service`](event-display.yaml) that the Scheduler notifications
   will sink into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

## Verify

We will verify that the published event was sent by looking at the logs of the
service that this Scheduler job sinks to.

1. We need to wait for the downstream pods to get started and receive our event,
   wait 60 seconds. You can check the status of the downstream pods with:

   ```shell
   kubectl get pods --selector app=event-display
   ```

   You should see at least one.

1. Inspect the logs of the `Service`:

   ```shell
   kubectl logs --selector app=event-display -c user-container --tail=200
   ```

You should see log lines similar to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: google.cloud.scheduler.job.v1.executed
  source: //cloudscheduler.googleapis.com/projects/test-project/locations/us-central1/jobs/cre-scheduler-a7155fae-895c-4d11-b555-b2cd5ed97666
  id: 1313918157507406
  time: 2020-06-30T16:21:00.861Z
  dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/scheduler/v1/data.proto
  datacontenttype: application/json
Extensions,
  knativearrivaltime: 2020-06-30T16:21:01.401511767Z
  knsourcetrigger: link0.16512790926262466
  traceparent: 00-37bb197929fc15a684be311da682fce2-4af58d9f16415e4a-00
Data,
  {
    "custom_data": "c2NoZWR1bGVyIGN1c3RvbSBkYXRh" // base64 encoded "scheduler custom data"
  }
```

## Troubleshooting

You may have issues receiving desired CloudEvent. Please use
[Authentication Mechanism Troubleshooting](../../how-to/authentication-mechanism-troubleshooting.md)
to check if it is due to an auth problem.

## What's Next

1. For more details on Cloud Pub/Sub formats refer to the
   [Subscriber overview guide](https://cloud.google.com/pubsub/docs/subscriber).
1. For integrating with Cloud Pub/Sub, see the
   [PubSub example](../../examples/cloudpubsubsource/README.md).
1. For integrating with Cloud Storage see the
   [Storage example](../../examples/cloudstoragesource/README.md).
1. For integrating with Cloud Audit Logs see the
   [Cloud Audit Logs example](../../examples/cloudauditlogssource/README.md).
1. For integrating with Cloud Build see the
   [Build example](../../examples/cloudbuildsource/README.md).
1. For more information about CloudEvents, see the
   [HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## Cleaning Up

1. Delete the `CloudSchedulerSource`

   ```shell
   kubectl delete -f ./cloudschedulersource.yaml
   ```

1. Delete the `Service`

   ```shell
   kubectl delete -f ./event-display.yaml
   ```
