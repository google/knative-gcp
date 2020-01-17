# Cloud Scheduler Example

## Overview

This sample shows how to Configure `Scheduler` resource for receiving scheduled
events from [Google Cloud Scheduler](https://cloud.google.com/scheduler/).

## Prerequisites

1. [Install Knative with GCP](../install/README.md). Note that your project needs to be created with
   an App Engine application. Refer to this [guide](https://cloud.google.com/scheduler/docs/quickstart#create_a_project_with_an_app_engine_app)
   for more details.

1. [Create a Pub/Sub enabled Service Account](../install/pubsub-service-account.md)

1. Enable the `Cloud Scheduler API` on your project:

   ```shell
   gcloud services enable cloudscheduler.googleapis.com
   ```

## Deployment

1. Create a [scheduler](./scheduler.yaml)

    ```shell
    kubectl apply --filename scheduler.yaml
    ```

1. Create a [service](./event-display.yaml) that the Scheduler notifications will sink into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

## Verify

We will verify that the published event was sent by looking at the logs of the
service that this Scheduler job sinks to.

1. We need to wait for the downstream pods to get started and receive our event,
   wait 60 seconds.

   - You can check the status of the downstream pods with:

     ```shell
     kubectl get pods --selector app=event-display
     ```

     You should see at least one.

1. Inspect the logs of the service:

   ```shell
   kubectl logs --selector app=event-display -c user-container
   ```

You should see log lines similar to:

```shell
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 1.0
  type: com.google.cloud.sheduler.job.execute
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

1. For integrating with Cloud Pub/Sub, see the [PubSub example](../pubsub/README.md).
1. For integrating with Cloud Storage see the [Storage example](../storage/README.md).
1. For more information about CloudEvents, see the [HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## Cleaning Up

1. Delete the Scheduler

    ```shell
    kubectl delete -f ./scheduler.yaml
    ```
1. Delete the Service    
    
    ```shell
    kubectl delete -f ./event-display.yaml
    ```
