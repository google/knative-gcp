# Cloud Scheduler Example

## Overview

This sample shows how to Configure `Scheduler` resource for receiving scheduled
events from [Google Cloud Scheduler](https://cloud.google.com/scheduler/) This
is a simple example that uses a single Service Account for manipulating both
Pub/Sub resources as well as Scheduler resources.

## Prerequisites

1. [Create a Pub/Sub enabled Service Account](../pubsub)

1. [Install Knative with GCP](../install).

1. Enable the `Cloud Scheduler API` on your project:

   ```shell
   gcloud services enable cloudscheduler.googleapis.com
   ```

1. Give that Service Account the `Cloud Scheduler Admin` role on your Google
   Cloud project:

   ```shell
   gcloud projects add-iam-policy-binding $PROJECT_ID \
   --member=serviceAccount:cloudrunevents-pullsub@$PROJECT_ID.iam.gserviceaccount.com \
   --role roles/cloudscheduler.admin
   ```

## Deployment

```shell
kubectl apply --filename scheduler.yaml
```

1. Create a service that the Scheduler notifications will sink into:

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
  specversion: 0.3
  type: com.google.cloud.pubsub.topic.publish
  source: //pubsub.googleapis.com/projects/quantum-reducer-434/topics/scheduler-a424c4a6-c9e2-11e9-a541-42010aa8000b
  id: 714614529367848
  time: 2019-08-28T22:29:00.979Z
  datacontenttype: application/json
Extensions,
  error: unexpected end of JSON input
  knativecemode: binary
Data,
  my test data
```

## What's Next

The Scheduler implements what Knative Eventing considers to be an `importer`.
This component can work alone, but it also works well when
[Knative Serving and Eventing](https://github.com/knative/docs) are installed in
the cluster.

## Cleaning Up

```shell
kubectl delete -f ./scheduler.yaml
kubectl delete -f ./event-display.yaml
gcloud projects remove-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:cloudrunevents-pullsub@$PROJECT_ID.iam.gserviceaccount.com \
  --role roles/cloudscheduler.admin
```
