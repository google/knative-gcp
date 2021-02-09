# Managing Multiple Projects

A general Knative-GCP installation configures three identities (represents as Google service accounts) on your project.

1. Identity for the Control Plane.
2. Identity for the GCP Broker.
3. Identity for the Source. (This is the default identity for the Source.
Users can use this identity for multiple Source custom objects,
or create additional identities in the form of Google service accounts for different Source custom objects,
to achieve more fine-grained configurations)

In general Knative-GCP installation guide, each Knative-GCP component consumes/creates Google Cloud resources within the scope of the project
(the project where the kubernetes cluster contained your Knative-GCP components resides). For example, if your kubernetes cluster is in Project A, and
you apply StorageSource in this kubernetes cluster, it will create a Google Cloud Storage Bucket, a Google Cloud PubSub Topic, and a Google Cloud PubSub Subscription,
all belong to Project A.

This topic explains how to set-up Knative GCP configuration to create Sources which consume/create Google Cloud resources from other projects.

Here we use PubSubSource as an example.

The following names are representing:
* `project-a`: Project (project where all Knative-GCP system resides)

* `events-controller-gsa@project-a.iam.gserviceaccount.com`: Control-plane GSA (which is used for the control plane)

* `project-b`: Project (project where your topic and subscription resides)

* `your-gsa@project-b.iam.gserviceaccount.com`: Source Data-Plane GSA (which is used in your namespace where your PubSubSource resides)

In order to make your PubSubSource in Project A' cluster to consume Google PubSub resource in Project B:
1. Control Plane GSA `events-controller-gsa@project-a.iam.gserviceaccount.com` needs `pubsub.editor` role for `project-b`
    ```
    gcloud projects add-iam-policy-binding project-b
    --member=serviceAccount:events-controller-gsa@project-a.iam.gserviceaccount.com
    --role roles/pubsub.editor
    ```
    ***Note:*** To create different Google Cloud Resource, the Control Plane needs different permissions.
    Refer to [Manually Configure Authentication Mechanism for the Control Plane](./authentication-mechanisms-gcp.md/#authentication-mechanism-for-the-control-plane)
    for specific roles(a set of permissions) needed to create each Source.

1. Source Data-plane GSA `your-gsa@project-b.iam.gserviceaccount.com` needs `pubsub.editor` role for `project-b`

    ```
    gcloud projects add-iam-policy-binding project-b
    --member=serviceAccount:your-gsa@project-b.iam.gserviceaccount.com
    --role roles/pubsub.editor
    ```
   ***Note:*** Source Data Plane only pulls subscription from Cloud PubSub Subscription, thus,
   no matter which Cloud Source you are creating, this GSA only needs `roles/pubsub.editor` role.
   Refer to [Installing a Service Account for the Data Plane](../install/dataplane-service-account.md)
   for specific roles(a set of permissions) needed if you also want metrics and tracing for your Source.

1. Configure credentials in the kubernetes cluster. You can either use:
    1. Workload Identity
        1. Create a Kubernetes Service Account `ksa-name` in the same namespace where the PubSubSource will reside.
            ```
            kubectl create service account ksa-name -n namespace
            ```
        2. Source Data-plane GSA `your-gsa@project-b.iam.gserviceaccount.com` needs workload identity binding.
            ```
            gcloud iam service-accounts add-iam-policy-binding your-gsa@project-b.iam.gserviceaccount.com
            --role "roles/iam.workloadIdentityUser"
            --member "serviceAccount:project-a.svc.id.goog[namespace/ksa-name]”
            ```
        3. Kubernetes Service Account `ksa-name` needs annotation
            ```
            kubectl annotate serviceaccount ksa-name \
            iam.gke.io/gcp-service-account=your-gsa@project-b.iam.gserviceaccount.com
            ```
    1. or Exporting Service Account Keys And Store Them as Kubernetes Secret
       1. Download a new JSON private key for Source Data-plane GSA `your-gsa@project-b.iam.gserviceaccount.com`

          ```shell
          gcloud iam service-accounts keys create events-sources-key.json \
            --iam-account=your-gsa@project-b.iam.gserviceaccount.com
          ```

       1. Create a secret on the Kubernetes cluster with the downloaded key. Remember
          to create the secret in the namespace your resources will reside.

          ```shell
          kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=events-sources-key.json
          ```

          `google-cloud-key` and `key.json` are default values expected by our
          resources.
1. Following [CloudPubSubSource](../examples/cloudpubsubsource/README.md) example to create your PubSubSource.
***[Important]*** Don't forget to specifying `project-b` and `secret`/ `serviceAccountName` under `spec` when you apply your PubSubSource:

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
      project: project-b
      ...
   ```
