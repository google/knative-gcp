# Topic Example

This sample shows how to configure `Topics`, which is our Kubernetes object to
represent Cloud Pub/Sub topics. This resource can be used to interact directly
with Cloud Pub/Sub. It has a HTTP-addressable endpoint, where users can POST
CloudEvents, which will be then published to Cloud Pub/Sub. This is a core
construct used by higher-level objects, such as `Channel`.

## Prerequisites

1. [Install Knative-GCP](../../install/install-knative-gcp.md)

1. [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md)

## Deployment

1. Create a Cloud Pub/Sub Topic.

   ```shell
   export TOPIC_NAME=testing
   gcloud pubsub topics create $TOPIC_NAME
   ```

1. Update `googleServiceAccount` / `secret` in the [`topic.yaml`](topic.yaml)

   1. If you are in GKE and using
      [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity),
      update `serviceAccountName` with the Kubernetes service account you
      created in
      [Create a Service Account for the Data Plane](../../install/dataplane-service-account.md),
      which is bound to the Pub/Sub enabled Google service account.

   1. If you are using standard Kubernetes secrets, but want to use a
      non-default one, update `secret` with your own secret.

1. Update `TOPIC_NAME` in the [`topic.yaml`](topic.yaml) and apply it.

   If you're in the topic directory, you can replace `TOPIC_NAME` and apply in
   one command:

   ```shell
   sed "s/\TOPIC_NAME/$TOPIC_NAME/g" topic.yaml | \
       kubectl apply --filename -
   ```

   If you are replacing `TOPIC_NAME` manually, then make sure you apply the
   resulting YAML:

   ```shell
   kubectl apply --filename topic.yaml
   ```

1. Create a Pod using [`curl`](curl.yaml), which will act as the event producer.

   ```shell
   kubectl apply --filename curl.yaml
   ```

1. Create a Cloud Pub/Sub subscription so that you can receive the message
   published:

   ```shell
   gcloud pubsub subscriptions create test-topic-subscription \
    --topic=$TOPIC_NAME \
    --topic-project=$PROJECT_ID
   ```

## Publish

1. You can publish an event by sending an HTTP request to the `Topic`. SSH into
   the `curl` Pod by running the following command:

   ```shell
   kubectl --namespace default attach curl -it
   ```

1. While in the Pod prompt, create an event with:

   ```shell
   curl -v "http://cre-testing-sample-publish.default.svc.cluster.local" \
   -X POST \
   -H "Ce-Id: my-id" \
   -H "Ce-Specversion: 1.0" \
   -H "Ce-Type: alpha-type" \
   -H "Ce-Source: my-source" \
   -H "Content-Type: application/json" \
   -d '{"msg":"send-cloudevents-to-topic"}'
   ```

   You should receive an HTTP 202 Accepted response.

## Verify

We will verify that the event was actually sent to Cloud Pub/Sub by pulling the
message from the subscription:

```shell
gcloud pubsub subscriptions pull test-topic-subscription --format=json
```

You should see log lines similar to:

```json
[
  {
    "ackId": "TgQhIT4wPkVTRFAGFixdRkhRNxkIaFEOT14jPzUgKEURBggUBXx9cltXdV8zdQdRDRlzemF0blhFAgZFB3RfURsfWVx-Sg5WDxpwfWhxaVgRAgdNUVa4koT9iuWxRB1tNfOWpKBASs3pifF0Zhs9XxJLLD5-NTlFQV5AEkw-DERJUytDCypYEQ",
    "message": {
      "attributes": {
        "ce-datacontenttype": "application/json",
        "ce-id": "my-id",
        "ce-source": "my-source",
        "ce-specversion": "1.0",
        "ce-time": "2020-02-05T07:10:02.602778258Z",
        "ce-traceparent": "00-3c20e18012cac372b2c0168b9d99268a-dedab6911074371a-01",
        "ce-type": "alpha-type"
      },
      "data": "eyJtc2ciOiJzZW5kLWNsb3VkZXZlbnRzLXRvLXRvcGljIn0=",
      "messageId": "972320943223582",
      "publishTime": "2020-02-05T07:10:02.893Z"
    }
  }
]
```

## Troubleshooting

You may have issues receiving desired CloudEvent. Please use
[Authentication Mechanism Troubleshooting](../../how-to/authentication-mechanism-troubleshooting.md)
to check if it is due to an auth problem.

## What's next

1. For a higher-level construct to interact with Cloud Pub/Sub that sends
   Push-compatible format events, see the
   [PubSub example](../../examples/cloudpubsubsource/README.md).
1. For integrating with Cloud Storage see the
   [Storage example](../../examples/cloudstoragesource/README.md).
1. For integrating with Cloud Scheduler see the
   [Scheduler example](../../examples/cloudschedulersource/README.md).
1. For integrating with Cloud Audit Logs see the
   [Cloud Audit Logs example](../../examples/cloudauditlogssource/README.md).
1. For more information about CloudEvents, see the
   [HTTP transport bindings documentation](https://github.com/cloudevents/spec).

## Cleaning Up

1. Delete the `Topic`

   If you're in the topic directory, you can replace `TOPIC_NAME` and delete in
   one command:

   ```shell
    sed "s/\TOPIC_NAME/$TOPIC_NAME/g" topic.yaml | \
        kubectl delete --filename -
   ```

   If you replaced `TOPIC_NAME` manually, then make sure you delete the
   resulting YAML:

   ```shell
   kubectl delete --filename topic.yaml
   ```

1. Delete the `curl` Pod used as the source:

   ```shell
   kubectl delete --filename curl.yaml
   ```
