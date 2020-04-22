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

## Configure the Authentication Mechanism for GCP

### Option 1: Use Workload Identity

It is the recommended way to access Google Cloud services from within GKE due to
its improved security properties and manageability. For more information about
Workload Identity see
[here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

1. Enable Workload Identity. Check [Manually Configure Authentication Mechanism for GCP](authentication-mechanisms-gcp.md) ->
   [Option 1 (Recommended): Workload Identity](authentication-mechanisms-gcp.md/#option-1-recommended-workload-identity) -> 
   Enable Workload Identity, for setup details. 
   
   Skip this step if you have already enabled it during the control plane setup:
   [Install Knative-GCP](install-knative-gcp.md). 
    
1. Give `iam.serviceAccountAdmin` role to your control plane's Google
  Cloud Service Account. You can skip this step if you granted
  `roles/owner` privileges during the control plane setup:
  [Install Knative-GCP](install-knative-gcp.md).

   ```shell
   gcloud projects add-iam-policy-binding $PROJECT_ID \
   --member=serviceAccount:cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
   --role roles/iam.serviceAccountAdmin
   ```

1. Update `spec.googleServiceAccount` with the
   [Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project)
   you just created when creating resources. Check
   [example](https://github.com/google/knative-gcp/tree/master/docs/examples) of
   each resource for detail.

   Generally, updating `spec.googleServiceAccount` will trigger the controller
   to enable Workload Identity for sources in the data plane. The following
   steps are handled by the controller:

   1. Create a Kubernetes Service Account with an `OwnerReference` of the
      source.

   1. Bind the Kubernetes Service Account with Google Cloud Service Account,
      this will add `role/iam.workloadIdentityUser` to the Google Cloud Service
      Account. The scope of this role is only for this specific Google Cloud
      Service Account. It is equivalent to this command:

      ```shell
      gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member serviceAccount:$PROJECT_ID.svc.id.goog[namespace/Kubernetes-service-account] \
      GCP-service-account@$PROJECT_ID.iam.gserviceaccount.com
      ```

   1. Annotate the Kubernetes Service Account with
      `iam.gke.io/gcp-service-account=GCP-service-account@PROJECT_ID.iam.gserviceaccount.com`

   For every namespace, the Kubernetes Service Account and the Google Cloud
   Service Account will have a one to one mapping. If multiple source instances
   use the same Google Cloud Service Account, they will use the same Kubernetes
   Service Account as well, and the Kubernetes Service Account will have
   multiple `OwnerReference`s. If all sources using a Kubernetes Service Account
   are deleted then that Kubernetes Service Account, and its corresponding
   Google Cloud Service account, will be deleted.

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
