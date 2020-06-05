# Installing Pub/Sub Enabled Service Account

Besides the control plane setup described in the general
[installation guide](./install-knative-gcp.md), each of our resources have a
data plane component, which basically needs permissions to read and/or write to
Pub/Sub. Herein, we show the steps needed to configure such Pub/Sub enabled
Service Account.

## Prerequisites

1. Create a
   [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   and install the `gcloud` CLI and run `gcloud auth login`. This sample will
   use a mix of `gcloud` and `kubectl` commands. The rest of the sample assumes
   that you've set the `$PROJECT_ID` environment variable to your Google Cloud
   project id, and also set your project ID as default using
   `gcloud config set project $PROJECT_ID`.

1. Enable the `Cloud Pub/Sub API` on your project:

   ```shell
   gcloud services enable pubsub.googleapis.com
   ```

## Create a [Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project) to interact with Pub/Sub

In general, we would just need permissions to receive messages
(`roles/pubsub.subscriber`). However, in the case of the `Channel`, we would
also need the ability to publish messages (`roles/pubsub.publisher`).

1. Create a new Service Account named `cre-pubsub` with the following command:

   ```shell
   gcloud iam service-accounts create cre-pubsub
   ```

1. Give that Service Account the necessary permissions on your project.

   In this example, and for the sake of simplicity, we will just grant
   `roles/pubsub.editor` privileges to the Service Account, which encompasses
   both of the above plus some other permissions. Note that if you prefer
   finer-grained privileges, you can just grant the ones mentioned above.

   ```shell
   gcloud projects add-iam-policy-binding $PROJECT_ID \
     --member=serviceAccount:cre-pubsub@$PROJECT_ID.iam.gserviceaccount.com \
     --role roles/pubsub.editor
   ```

## Configure the Authentication Mechanism for GCP (the Data Plane)

### Option 1: Use Workload Identity

It is the recommended way to access Google Cloud services from within GKE due to
its improved security properties and manageability. For more information about
Workload Identity see
[here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

**_Note:_** If you install the Knative-GCP Constructs with v0.14.0 or older
release, please use option 2.   
**_Note:_** `spec.googleServiceAccount` in v0.14.0 is deprecated for security
implications. It has not been promoted to v1beta1 and is expected to be removed
from v1alpha1 in the v0.16.0 release. Instead, `spec.serviceAccountName` has
been introduced for Workload Identity in v0.15.0, whose value is a Kubernetes
Service Account.

Using the Google Cloud Service Account you just created and checking
[Option 1 (Recommended): Workload Identity](../install/authentication-mechanisms-gcp.md/#option-1-recommended-workload-identity)
in
[Manually Configure Authentication Mechanism for GCP](../install/authentication-mechanisms-gcp.md)
to configure Workload Identity in the namespace your resources will reside.

You will have a Kubernetes Service Account after the above configuration, which
is bound to the Google Cloud Service Account you just created. Remember to put
this Kubernetes Service Account name under `spec.serviceAccountName` when you
follow
[example](https://github.com/google/knative-gcp/tree/master/docs/examples) to
use resource.

### Option 2. Export Service Account Keys And Store Them as Kubernetes Secrets

1. Download a new JSON private key for that Service Account. **Be sure not to
   check this key into source control!**

   ```shell
   gcloud iam service-accounts keys create cre-pubsub.json \
   --iam-account=cre-pubsub@$PROJECT_ID.iam.gserviceaccount.com
   ```

1. Create a secret on the Kubernetes cluster with the downloaded key. Remember
   to create the secret in the namespace your resources will reside. The example
   below does so in the `default` namespace.

   ```shell
   kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=cre-pubsub.json
   ```

   `google-cloud-key` and `key.json` are default values expected by our
   resources.

## Cleaning Up

1. Delete the secret

   ```shell
   kubectl --namespace default delete secret google-cloud-key
   ```

1. Delete the service account

   ```shell
   gcloud iam service-accounts delete cre-pubsub@$PROJECT_ID.iam.gserviceaccount.com
   ```
