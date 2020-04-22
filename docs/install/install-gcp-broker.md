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

It is the recommended way to access Google Cloud services from within
GKE due to its improved security properties and manageability. For more
information about Workload Identity see
[here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

1. Set your project ID.

    ```shell
    export PROJECT_ID=<your-project-id>
    ```

1. Make sure Workload Identity is enabled when you [Install Knative-GCP](install-knative-gcp.md).
If not, [configure authentication mechanism](authentication-mechanisms-gcp.md)
using the Workload Identity option.
 
1. Bind the `broker` Kubernetes Service Account with Google Cloud Service
Account, this will add `role/iam.workloadIdentityUser` to the
Google Cloud Service Account. The scope of this role is only for
this specific Google Cloud Service Account. It is equivalent to this
command:

    ```shell
    gcloud iam service-accounts add-iam-policy-binding \
    --role roles/iam.workloadIdentityUser \
    --member serviceAccount:$PROJECT_ID.svc.id.goog[cloud-run-events/broker] \
    cre-pubsub@$PROJECT_ID.iam.gserviceaccount.com
    ```

1. Annotate the `broker` Kubernetes Service Account with
`iam.gke.io/gcp-service-account=cre-pubsub@PROJECT_ID.iam.gserviceaccount.com`

    ```shell
    kubectl --namespace cloud-run-events  annotate serviceaccount broker iam.gke.io/gcp-service-account=cre-pubsub@${PROJECT_ID}.iam.gserviceaccount.com
    ```

### Option 2.  Export Service Account Keys And Store Them as Kubernetes Secrets

1. Download a new JSON private key for that Service Account. **Be sure
    not to check this key into source control!**

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
