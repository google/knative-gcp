# CloudPubSubSource Example

## Overview

This sample shows how to configure the CloudPubSubSource. 
This source is most useful as a bridge from other GCP services, 
such as [Cloud Storage](https://cloud.google.com/storage/docs/pubsub-notifications), 
[Cloud Scheduler](https://cloud.google.com/scheduler/docs/creating#).

## Prerequisites

1. [Install Knative with GCP](../../install/README.md).

1. [Create a Pub/Sub enabled Service Account](../../install/pubsub-service-account.md)

## Deployment

1. Create a GCP PubSub Topic. If you change its name (`testing`), you also need
      to update the `topic` in the
      [`CloudPubSubSource`](cloudpubsubsource.yaml) file:
   
      ```shell
      gcloud pubsub topics create testing
      ``` 
     
1. Create a [`CloudPubSubSource`](cloudpubsubsource.yaml)
 
     ```shell
     kubectl apply --filename cloudpubsubsource.yaml
     ```
      
1. Create a [`Service`](event-display.yaml) that the CloudAuditLogsSource will sink into:

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

1. You can check the status of the downstream pods with:

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
  source: //pubsub.googleapis.com/projects/xiyue-knative-gcp/topics/testing
  id: 946366448650699
  time: 2020-01-21T22:12:06.742Z
  datacontenttype: application/octet-stream
Extensions,
  knativecemode: binary
Data,
  {"Hello": "world"}
```

## Cleaning Up

1. Delete the `CloudPubSubSource`

    ```shell
    kubectl delete -f ./cloudpubsubsource.yaml
    ```
1. Delete the `Service`  
    
    ```shell
    kubectl delete -f ./event-display.yaml
    ```
