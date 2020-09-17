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

### Option 1: Use Workload Identity

It is the recommended way to access Google Cloud services from within GKE due to
its improved security properties and manageability. For more information about
Workload Identity see
[here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

**_Note:_**

1. If you install the Knative-GCP Constructs with v0.14.0 or older release,
   please use option 2.
2. `spec.googleServiceAccount` in v0.14.0 is deprecated for security
   implications. It has not been promoted to v1beta1 and is expected to be
   removed from v1alpha1 in the v0.16.0 release. Instead,
   `spec.serviceAccountName` has been introduced for Workload Identity in
   v0.15.0, whose value is a Kubernetes Service Account.

There are two scenarios to leverage Workload Identity for resources in the Data
Plane:

- **_Non-default scenario:_**

  Using the Google Cloud Service Account `cre-dataplane` you just created and
  using
  [Option 1 (Recommended): Workload Identity](../install/authentication-mechanisms-gcp.md/#option-1-recommended-workload-identity)
  in
  [Authentication Mechanism for GCP](../install/authentication-mechanisms-gcp.md)
  to configure Workload Identity in the namespace your resources will reside.
  (You may notice that this link is pointing to the manual Workload Identity
  configuration in the Control Plane. Non-default scenario Workload Identity
  configuration in the Data Plane is similar to the manual Workload Identity
  configuration in the Control Plane)

  You will have a Kubernetes Service Account after the above configuration,
  which is bound to the Google Cloud Service Account `cre-dataplane`. Remember
  to put this Kubernetes Service Account name as the `spec.serviceAccountName`
  when you create resources in the
  [example](https://github.com/google/knative-gcp/tree/master/docs/examples).

- **_Default scenario:_**

  Instead of manually configuring Workload Identity with
  [Authentication Mechanism for GCP](../install/authentication-mechanisms-gcp.md),
  you can authorize the Controller to configure Workload Identity for you.

  You need to grant `iam.serviceAccountAdmin` permission of the Google Cloud
  Service Account `cre-dataplane` you just created to the Control Plane's Google
  Cloud Service Account `cloud-run-events` by:

  ```shell
  gcloud iam service-accounts add-iam-policy-binding \
   cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com  \
   --member=serviceAccount:cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
   --role roles/iam.serviceAccountAdmin
  ```

  Then, modify `clusterDefaults` in ConfigMap `config-gcp-auth`.

  You can directly edit the ConfigMap by:

  ```shell
  kubectl edit configmap config-gcp-auth -n cloud-run-events
  ```

  and add `workloadIdentityMapping` in `clusterDefaults`:

  ```shell
  default-auth-config: |
    clusterDefaults:
      workloadIdentityMapping:
        default-cre-dataplane: cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com
  ```

  When updating the configuration, note that `default-auth-config` is nested
  under `data`. If you encounter an error, you are likely attempting to modify
  the example configuration in `_example`.

  Here, `default-cre-dataplane` refers to a Kubernetes Service Account bound to
  the Google Cloud Service Account `cre-dataplane`. Remember to put this
  Kubernetes Service Account name as the `spec.serviceAccountName` when you
  create resources in the
  [example](https://github.com/google/knative-gcp/tree/master/docs/examples).

  Kubernetes Service Account `default-cre-dataplane` doesn't need to exist in a
  specific namespace. Once it is set in the ConfigMap `config-gcp-auth`， the
  Control Plane will create it for you and configure the corresponding Workload
  Identity relationship between the Kubernetes Service Account
  `default-cre-dataplane` and the Google Cloud Service Account `cre-dataplane`
  when you create resources using the Kubernetes Service Account
  `default-cre-dataplane`.

A `Condition` `WorkloadIdentityConfigured` will show up under resources'
`Status`, indicating the Workload Identity configuration status.  
 **_Note:_** The Controller currently doesn’t perform any access control checks,
as a result, any user who can create a resource can get access to the Google Cloud
Service Account which grants the `iam.serviceAccountAdmin` permission to the Controller.
As an example, if you followed the instructions above, then any user that can make
a Knative-GCP source or Channel (e.g. `CloudAuditLogsSource`, `CloudPubSubSource`,
etc.) can cause the Kubernetes Service Account `default-cre-dataplane` to be created.
If they can also create Pods in that namespace, then they can make a Pod that uses
the Google Service Account `cre-dataplane` credentials.

### Option 2. Export Service Account Keys And Store Them as Kubernetes Secrets

1. Download a new JSON private key for that Service Account. **Be sure not to
   check this key into source control!**

   ```shell
   gcloud iam service-accounts keys create cre-dataplane.json \
   --iam-account=cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com
   ```

1. Create a secret on the Kubernetes cluster with the downloaded key. Remember
   to create the secret in the namespace your resources will reside. The example
   below does so in the `default` namespace.

   ```shell
   kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=cre-dataplane.json
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
   gcloud iam service-accounts delete cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com
   ```
