# CloudSchedulerSource Example

## Overview

This sample shows how to Configure `CloudSchedulerSource` resource for receiving
scheduled events from
[Google Cloud Scheduler](https://cloud.google.com/scheduler/).

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md). Note that your
   project needs to be created with an App Engine application. Refer to this
   [guide](https://cloud.google.com/scheduler/docs/quickstart#create_a_project_with_an_app_engine_app)
   for more details.

1. [Create a Pub/Sub enabled Service Account](../../install/pubsub-service-account.md)

1. Enable the `Cloud Scheduler API` on your project:

   ```shell
   gcloud services enable cloudscheduler.googleapis.com
   ```

## Deployment

1. Create a [`CloudSchedulerSource`](cloudschedulersource.yaml)

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `googleServiceAccount` with the Pub/Sub enabled service account you
      created in
      [Create a Pub/Sub enabled Service Account](../../install/pubsub-service-account.md).

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
   kubectl logs --selector app=event-display -c user-container
   ```

You should see log lines similar to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: com.google.cloud.scheduler.job.execute
  source: //cloudscheduler.googleapis.com/projects/knative-gcp/locations/us-east4/schedulers/scheduler-test
  subject: jobs/cre-scheduler-bfc82b00-11fd-42ec-b21a-011dddc2170b
  id: 714614529367848
  time: 2019-08-28T22:29:00.979Z
  datacontenttype: application/octet-stream
Extensions,
  knativecemode: binary
Data,
  my test data
```

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
