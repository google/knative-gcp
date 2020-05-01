# Installing GCP Broker

## GCP Broker

TODO([#887](https://github.com/google/knative-gcp/issues/887)) Add description
for GCP broker and how is it different from the channel based broker.

## Prerequisites

1. [Install Knative-GCP](./install-knative-gcp.md).

2. [Create a Pub/Sub enabled Service Account](./pubsub-service-account.md).

## Deployment

Apply GCP broker yamls:

```shell
ko apply -f ./config/broker/
```

## Authentication Setup for GCP Broker

### Option 1: Use Workload Identity

It is the recommended way to access Google Cloud services from within GKE due to
its improved security properties and manageability. For more information about
Workload Identity see
[here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

1.  Set your project ID.

    ```shell
    export PROJECT_ID=<your-project-id>
    ```

1.  Make sure Workload Identity is enabled when you
    [Install Knative-GCP](install-knative-gcp.md). If not,
    [configure authentication mechanism](authentication-mechanisms-gcp.md) using
    the Workload Identity option.

1.  Bind the `broker` Kubernetes Service Account with Google Cloud Service
    Account, this will add `role/iam.workloadIdentityUser` to the Google Cloud
    Service Account. The scope of this role is only for this specific Google
    Cloud Service Account. It is equivalent to this command:

        ```shell
        gcloud iam service-accounts add-iam-policy-binding \
        --role roles/iam.workloadIdentityUser \
        --member serviceAccount:$PROJECT_ID.svc.id.goog[cloud-run-events/broker] \
        cre-pubsub@$PROJECT_ID.iam.gserviceaccount.com
        ```

1.  Annotate the `broker` Kubernetes Service Account with
    `iam.gke.io/gcp-service-account=cre-pubsub@PROJECT_ID.iam.gserviceaccount.com`

        ```shell
        kubectl --namespace cloud-run-events  annotate serviceaccount broker iam.gke.io/gcp-service-account=cre-pubsub@${PROJECT_ID}.iam.gserviceaccount.com
        ```

### Option 2. Export Service Account Keys And Store Them as Kubernetes Secrets

1. Download a new JSON private key for that Service Account. **Be sure not to
   check this key into source control!**

   ```shell
   gcloud iam service-accounts keys create cre-pubsub.json \
   --iam-account=cre-pubsub@$PROJECT_ID.iam.gserviceaccount.com
   ```

1. Create a secret on the Kubernetes cluster with the downloaded key.

   ```shell
   kubectl --namespace cloud-run-events create secret generic google-broker-key --from-file=key.json=cre-pubsub.json
   ```

   `google-broker-key` and `key.json` are default values expected by our
   resources.

## Usage

```shell
export NAMESPACE=cloud-run-events-example
export BROKER=test-broker
```

1. Create a New GCP Broker

   ```shell
   kubectl apply -f - << END
   apiVersion: eventing.knative.dev/v1beta1
   kind: Broker
   metadata:
   name: ${BROKER}
   namespace: ${NAMESPACE}
   annotations:
       "eventing.knative.dev/broker.class": "googlecloud"
   END
   ```

   Make sure you apply the annotation
   `"eventing.knative.dev/broker.class": "googlecloud"`. That triggers the
   knative-gcp to create a new GCP broker.

1. Verify that the new broker is running,

   ```shell
   kubectl -n ${NAMESPACE} get broker ${BROKER}
   ```

   This shows the broker you just created like so:

   ```shell
   NAME          READY   REASON   URL                                                                                             AGE
   test-broker   True             http://broker-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-example/test-broker   15s
   ```

Once the GCP broker is ready, you can use it by sending events to its `URL` and
create [Triggers](https://knative.dev/docs/eventing/broker-trigger/#trigger) to
receive events from it, just like any Knative Eventing
[Brokers](https://knative.dev/docs/eventing/broker-trigger/#broker).

You can find demos of the GCP broker in the
[examples](../examples/gcpbroker/README.md).
