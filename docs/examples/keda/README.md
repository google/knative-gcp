# KEDA-based AutoScaling 

## Overview

[KEDA](https://keda.sh/) is a Kubernetes-based Event Driven Autoscaler that drives scaling of any container in Kubernetes. 
It is especially interesting for our Pull-based Sources, as it has [native support](https://keda.sh/scalers/gcp-pub-sub/) 
for Pub/Sub.

In order to make any of the Knative-GCP Sources scale with KEDA, users need to create their Sources with the following annotation:

```yaml
metadata:
  annotations:
    autoscaling.knative.dev/class: keda.autoscaling.knative.dev
```

We also support the following options for fine-grained scaling configuration:

```yaml
metadata:
  annotations:
    autoscaling.knative.dev/class: keda.autoscaling.knative.dev
    autoscaling.knative.dev/minScale: "0"                # <-- minimum number of pods to scaled down to.
    autoscaling.knative.dev/maxScale: "5"                # <-- maximum number of pods to scaled out to.
    keda.autoscaling.knative.dev/pollingInterval: "30"   # <-- interval in seconds to poll metrics.
    keda.autoscaling.knative.dev/cooldownPeriod: "60"    # <-- period of inactivity in seconds before scaling down.  
    keda.autoscaling.knative.dev/subscriptionSize: "15"  # <-- number of undelivered messages in the subscription used to scale.
```
 

_Disclaimers:_ 

- This is still experimental and subject to change.
- KEDA uses StackDriver to collect metrics. As it can take a while until those metrics are "refreshed" in StackDriver, you 
might experience certain delay (~2-3 mins) until your Sources are properly scaled.
- If you have a latency-critical workload, then you might be better off by disabling the scale down to zero (i.e., set the `minScale` annotation to 1).


## Example

In this section we provide an example that leverages KEDA-based scaling to make the `CloudPubSubSource` scalable. Note that 
you could do this for any of the [Sources in Knative-GCP](../../../README.md), and not just `CloudPubSubSource`.

### Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Pub/Sub enabled Service Account](../../install/pubsub-service-account.md)

1. Given that KEDA queries StackDriver for metrics, give the Service Account created in the previous step permissions to do so.

    ```shell
    gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member=serviceAccount:cre-pubsub@$PROJECT_ID.iam.gserviceaccount.com \
      --role roles/monitoring.viewer
    ```

1. [Install KEDA](https://keda.sh/deploy/). Note that this example was tested using KEDA [v1.2.0](https://github.com/kedacore/keda/releases/tag/v1.2.0). 


### Deployment

1. Create a GCP PubSub Topic. If you change its name (`testing-keda`), you also need
   to update the `topic` in the [`CloudPubSubSource`](cloudpubsubsource-keda.yaml)
   file:

   ```shell
   gcloud pubsub topics create testing-keda
   ```

1. Create a scalable [`CloudPubSubSource`](cloudpubsubsource-keda.yaml)

   ```shell
   kubectl apply --filename cloudpubsubsource-keda.yaml
   ```
   
   This will create a `CloudPubSubSource` that can scale all the way down to zero and scale out to a maximum of 5 Pods.
   
1. Wait for a few seconds, and check that the `Deployment` created when the Source was instantiated has zero available replicas:

   ```shell
   kubectl get deployments -l pubsub.cloud.google.com/pullsubscription=cloudpubsubsource-keda-test
   ```  
   
   This shows that we have successfully scaled down to zero.
   
1. Create a [`Service`](event-display.yaml) that the CloudPubSubSource will
   sink into:

   ```shell
   kubectl apply --filename event-display.yaml
   ```

### Publish

Now, let's publish some messages to our GCP Pub/Sub topic, so that the `CloudPubSubSource` scales out.

```shell
for x in {1..50}; do gcloud pubsub topics publish testing-keda --message "Test Message ${x}"; done
```

### Verify

1. Verify that the `CloudPubSubSource` scales out to a maximum of 5 replicas (as configured in the yaml file), and then
scales back down to zero. Again, we will check for the active replica count of the `Deployment` that was created when the Source was instantiated.

   ```shell
   kubectl get deployments -l pubsub.cloud.google.com/pullsubscription=cloudpubsubsource-keda-test --watch
   ```    

    As mentioned in _Disclaimers_, note that it might take a few minutes (around 2-3 mins) until the metrics are updated in StackDriver. 
    You should see the number of available replicas increasing after that time. Also, after a few minutes all the messages 
    are received, you should see the number of available replicas going back to zero. 

1. Open a separate console to verify that the published messages were actually sent by looking at the logs of the service
that this `CloudPubSubSource` sinks to. Inspect the logs of the `Service`:

   ```shell
   kubectl logs --selector app=event-display -c user-container
   ```

    You should see multiple log lines similar to:
    
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
          "data": "VGVzdCBNZXNzYWdlIDQ4",
          "publishTime": "2020-01-24T18:29:36.874Z"
        }
      }
    ```

### Cleaning Up

1. Delete the `CloudPubSubSource`

   ```shell
   kubectl delete -f ./cloudpubsubsource-keda.yaml
   ```

1. Delete the `Service`

   ```shell
   kubectl delete -f ./event-display.yaml
   ```
