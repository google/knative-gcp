# Installing a Service Account for the Data Plane

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

1. Create a new Service Account named `cre-dataplane` with the following
   command:

   ```shell
   gcloud iam service-accounts create cre-dataplane
   ```

1. Give that Service Account the necessary permissions on your project.

   In this example, and for the sake of simplicity, we will just grant
   `roles/pubsub.editor` privileges to the Service Account, which encompasses
   both of the above plus some other permissions. Note that if you prefer
   finer-grained privileges, you can just grant the ones mentioned above.

   ```shell
   gcloud projects add-iam-policy-binding $PROJECT_ID \
     --member=serviceAccount:cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com \
     --role roles/pubsub.editor
   ```

   **_Note:_** If you are going to use metrics and tracing to track your
   resources, you also need `roles/monitoring.metricWriter` for metrics
   functionality:

   ```shell
   gcloud projects add-iam-policy-binding $PROJECT_ID \
     --member=serviceAccount:cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com \
     --role roles/monitoring.metricWriter
   ```
   and `roles/cloudtrace.agent` for tracing functionality:

   ```shell
   gcloud projects add-iam-policy-binding $PROJECT_ID \
     --member=serviceAccount:cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com \
     --role roles/cloudtrace.agent
   ```

## Configure the Authentication Mechanism for GCP (the Data Plane)

If you want to run [example](https://github.com/google/knative-gcp/tree/master/docs/examples)
to create resources (like [CloudPubSubSource](../examples/cloudpubsubsource/README.md),
[GloudSchedulerSource](../examples/cloudschedulersource/README.md), etc.) and make your resources'
Data Plane work, you need to make authentication configuration in the namespace where your resources reside.

Currently, we support two methods: Workload Identity and Kubernetes Secret. The
configuration steps have been automated by the scripts below. If you wish to
configure the auth manually, refer to
[Manually Configure Authentication Mechanism for the Data Plane](./authentication-mechanisms-gcp.md/#authentication-mechanism-for-the-data-plane).

Before applying initialization scripts, make sure:

1. Your default zone is set to be the same as your current cluster. You may use
   `gcloud container clusters describe $CLUSTER_NAME` to get zone and apply
   `gcloud config set compute/zone $ZONE` to set it.
1. Your gcloud `CLI` are up to date. You may use `gcloud components update` to
   update it.
### Option 1: Use Workload Identity

It is the recommended way to access Google Cloud services from within GKE due to
its improved security properties and manageability. For more information about
Workload Identity see
[here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

**_Note:_**

1. If you install the Knative-GCP Constructs with v0.14.0 or older release,
   please use option 2.
2. We assume that you have already [enabled Workload Identity](../install/authentication-mechanisms-gcp.md/#option-1-recommended-workload-identity)
   in your cluster.
3. `spec.googleServiceAccount` in v0.14.0 is deprecated for security
   implications. It has not been promoted to v1beta1 and is expected to be
   removed from v1alpha1 in the v0.16.0 release. Instead,
   `spec.serviceAccountName` has been introduced for Workload Identity in
   v0.15.0, whose value is a Kubernetes Service Account.

There are two scenarios to leverage Workload Identity for resources in the Data
Plane:

- **_Non-default scenario:_**

  Apply [init_data_plane_gke.sh](../../hack/init_data_plane_gke.sh) with parameters:

  ```shell
  ./hack/init_data_plane_gke.sh [MODE] [NAMESPACE] [K8S_SERVICE_ACCOUNT] [PROJECT_ID]
  ```
  Parameters available:

  1. `MODE`: an optional parameter to specify the mode to use, default to `default`.
  1. `NAMESPACE`: an optional parameter to specify the namespace to use, default to `default`.
  If the namespace does not exist, the script will create it.
  1. `K8S_SERVICE_ACCOUNT`: an optional parameter to specify the k8s service account to use, default to `default-cre-dataplane`.
  If the k8s service account does not exist, the script will create it.
  1. `PROJECT_ID`: an optional parameter to specify the project to use, default to
     `gcloud config get-value project`.

  Here is an example to run this script if you want to configure non-default Workload Identity
  in namespace `example` with Kubernetes service account `example-ksa`:

  ```shell
  ./hack/init_data_plane_gke.sh non-default example example-ksa
  ```

  After running the script, you will have a Kubernetes Service Account `example-ksa` in namespace `example`
  which is bound to the Google Cloud Service Account `cre-dataplane` (you just created it in the last step).
  Remember to put this Kubernetes Service Account name as the `spec.serviceAccountName`
  when you create resources in the
  [example](https://github.com/google/knative-gcp/tree/master/docs/examples).

- **_Default scenario:_**

  Instead of manually configuring Workload Identity namespace by namespace,
  you can authorize the Controller to configure Workload Identity for you.

  Apply [init_data_plane_gke.sh](../../hack/init_data_plane_gke.sh) without parameters:

  ```shell
  ./hack/init_data_plane_gke.sh
  ```

  After running this, every time when you create resources,
  the Controller will create a Kubernetes service account `default-cre-dataplane` in the
  namespace where your resources reside, and this Kubernetes service account
  is bound to the Google Cloud Service Account `cre-dataplane` (you just created it in the last step).
  What's more, you don't need to put this Kubernetes Service Account name as the `spec.serviceAccountName`
  when you create resources in the
  [example](https://github.com/google/knative-gcp/tree/master/docs/examples), the Controller will add it for you.

  A `Condition` `WorkloadIdentityConfigured` will show up under resources'
  `Status`, indicating the Workload Identity configuration status.

  **_Note:_** The Controller currently doesn’t perform any access control checks,
as a result, the Controller will configure Workload Identity (using Google Service Account
`cre-dataplane`'s credential) for any user who can create a resource.

### Option 2. Export Service Account Keys And Store Them as Kubernetes Secrets

Apply [init_data_plane.sh](../../hack/init_data_plane.sh) with parameters:

```shell
./hack/init_data_plane.sh [NAMESPACE] [SECRET] [PROJECT_ID]
```
Parameters available:
1.  `NAMESPACE`: an optional parameter to specify the namespace to use, default to `default`.
If the namespace does not exist, the script will create it.
1.  `SECRET`: an optional parameter to specify the secret, default to `google-cloud-key`.
If the secret does not exist, the script will create it.
1.  `PROJECT_ID`: an optional parameter to specify the project to use, default
    to `gcloud config get-value project`. If you want to specify the parameter
    `PROJECT_ID` instead of using the default one.

Here is an example to run this script if you want to configure authentication
in namespace `example` with the default secret name `google-cloud-key`

```shell
./hack/init_data_plane.sh example
```

After running the script, you will have a Kubernetes Secret `google-cloud-key`
in namespace `example` which stores the key exported from the Google Cloud service account
`cre-dataplane`(you just created it in the last step).
