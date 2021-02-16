# Managing Multiple Projects

A general Knative-GCP installation configures three identities (represents as
Google service accounts) on your project.

1. Identity for the Control Plane.
2. Identity for the GCP Broker.
3. Identity for the Source. (This is the default identity for the Source. Users
   can use this identity for multiple Source custom objects, or create
   additional identities in the form of Google service accounts for different
   Source custom objects, to achieve more fine-grained configurations).

In general Knative-GCP installation guide, each Knative-GCP component
consumes/creates Google Cloud resources within the scope of the project (the
project where the Kubernetes cluster containing your Knative-GCP components
resides). For example, if your Kubernetes cluster is in Project A, and you apply
[CloudStorageSource](../examples/cloudstoragesource) in this Kubernetes cluster,
then the Google Cloud Storage Notification, Google Cloud PubSub Topic, and
Google Cloud PubSub Subscription will be created in Project A.

This guide explains how to set-up Knative GCP configuration to create Sources
which consume/create Google Cloud resources from other projects.

Here we use CloudPubSubSource as an example.

We will use the following values as examples, you should replace them with the
appropriate values for your project when running the example commands:

```
# Cluster project is the GCP project that the Kubernetes cluster is running in.
export CLUSTER_PROJECT=project-a

# Source project is the GCP project that the source is reading information from.
export SOURCE_PROJECT=project-b

# Control plane GSA is the GSA that is being used by the control plane. Note that it is normally in the CLUSTER_PROJECT.
export CONTROL_PLANE_GSA="events-controller-gsa@$CLUSTER_PROJECT.iam.gserviceaccount.com"

# Source data plane GSA is the GSA that is used when reading from the source's PullSubscription. Note that it is normally in the SOURCE_PROJECT.
export SOURCE_DATA_PLANE_GSA="your-gsa@$SOURCE_PROJECT.iam.gserviceaccount.com"

# Namespace is the Kubernetes cluster namespace where the CloudPubSubSource resides.
export NAMESPACE=default

# Kubernetes service account name is the KSA that is used for the CloudPubSubSource's underlying Pods. Note that it is only used in Workload Identity.
export KSA_NAME=ksa-name
```

In order to make your CloudPubSubSource in `CLUSTER_PROJECT`'s cluster consume
Google PubSub resources in `SOURCE_PROJECT`:

1.  Control Plane GSA `CONTROL_PLANE_GSA` needs `pubsub.editor` role from
    `SOURCE_PROJECT`

    ```
    gcloud projects add-iam-policy-binding $SOURCE_PROJECT
    --member=serviceAccount:$CONTROL_PLANE_GSA
    --role roles/pubsub.editor
    ```

    **_Note:_** To create different Google Cloud Resource, the Control Plane
    needs different permissions. Refer to
    [Manually Configure Authentication Mechanism for the Control Plane](./authentication-mechanisms-gcp.md/#authentication-mechanism-for-the-control-plane)
    for specific roles (a set of permissions) needed to create each Source.

1.  Source Data Plane GSA `SOURCE_DATA_PLANE_GSA` need several roles from
    `SOURCE_PROJECT`

    1.  Needs `pubsub.editor` role to read information.
        ```
        gcloud projects add-iam-policy-binding $SOURCE_PROJECT
        --member=serviceAccount:$SOURCE_DATA_PLANE_GSA \
        --role roles/pubsub.editor
        ```
    1.  Needs `monitoring.metricWriter` and `cloudtrace.agent` role for metrics
        and tracing. ``` gcloud projects add-iam-policy-binding
        $SOURCE_PROJECT \
        --member=serviceAccount:$SOURCE_DATA_PLANE_GSA
        \
         --role roles/monitoring.metricWriter

             gcloud projects add-iam-policy-binding $SOURCE_PROJECT \
             --member=serviceAccount:$SOURCE_DATA_PLANE_GSA \
             --role roles/cloudtrace.agent
             ```

        **_Note:_** Source Data Plane only pulls subscription from Cloud PubSub
        Subscription, thus, no matter which CloudSource you create, the
        permissions needed are the same.

1.  Configure credentials in the Kubernetes cluster. You can either use:

    1. Workload Identity
       1. Create a Kubernetes Service Account `KSA_NAME` in the same namespace
          where the CloudPubSubSource will reside.
          ```
          kubectl create service account $KSA_NAME -n $NAMESPACE
          ```
       2. Source Data Plane GSA `SOURCE_DATA_PLANE_GSA` needs workload identity
          binding.
          ```
          gcloud iam service-accounts add-iam-policy-binding $SOURCE_DATA_PLANE_GSA
          --role "roles/iam.workloadIdentityUser"
          --member "serviceAccount:$CLUSTER_PROJECT.svc.id.goog[$NAMESPACE/$KSA_NAME]”
          ```
       3. Kubernetes Service Account `KSA_NAME` needs the workload identity
          annotation
          ```
          kubectl annotate serviceaccount $KSA_NAME \
          iam.gke.io/gcp-service-account=$SOURCE_DATA_PLANE_GSA
          ```
    1. or Exporting Service Account Keys And Store Them as Kubernetes Secret

       1. Download a new JSON private key for Source Data Plane GSA
          `SOURCE_DATA_PLANE_GSA`

          ```shell
          gcloud iam service-accounts keys create events-sources-key.json \
            --iam-account=$SOURCE_DATA_PLANE_GSA
          ```

       1. Create a secret on the Kubernetes cluster with the downloaded key.
          Remember to create the secret in the namespace where your
          CloudPubSubSource will reside.

          ```shell
          kubectl --namespace $NAMESPACE create secret generic google-cloud-key --from-file=key.json=events-sources-key.json
          ```

          `google-cloud-key` and `key.json` are default values expected by our
          resources.

1.  Follow [CloudPubSubSource](../examples/cloudpubsubsource/README.md) example
    to create your CloudPubSubSource. **_[Important]_** Don't forget to
    specifying `project` and `secret`/`serviceAccountName` under `spec` when you
    apply your CloudPubSubSource:

        ```
        apiVersion: events.cloud.google.com/v1
        kind: CloudPubSubSource
        metadata:
          name: cloudpubsubsource-test
        spec:
          ...
          secret:
            name: google-cloud-key
            key: key.json
          serviceAccountName: ksa-name
          project: $SOURCE_PROJECT
          ...

    ```

    ```
