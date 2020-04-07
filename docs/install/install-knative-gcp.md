# Installing Knative-GCP

1. Create a
   [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   and install the `gcloud` CLI and run `gcloud auth login`. This guide will use
   a mix of `gcloud` and `kubectl` commands. The rest of the guide assumes that
   you've set the `PROJECT_ID` environment variable to your Google Cloud project
   id, and also set your project ID as default using
   `gcloud config set project $PROJECT_ID`.

1. Install [Knative](https://knative.dev/docs/install/). Preferably, set up both
   [Serving](https://knative.dev/docs/serving/) and
   [Eventing](https://knative.dev/docs/eventing/). The latter is only required
   if you want to use the Pub/Sub `Channel` or a `Broker` backed by a Pub/Sub
   `Channel`.

1. Install the Knative-GCP constructs. You can either:

   - Install from master using [ko](http://github.com/google/ko)  
      `shell ko apply -f ./config` OR
   - Install a [release](https://github.com/google/knative-gcp/releases).
     Remember to update `{{< version >}}` in the commands below with the
     appropriate release version.

     1. First install the CRDs by running the `kubectl apply` command with the
        `--selector` flag. This prevents race conditions during the install,
        which cause intermittent errors:

        ```shell
        kubectl apply --selector pubsub.cloud.google.com/crd-install=true \
        --filename https://github.com/google/knative-gcp/releases/download/{{< version >}}/cloud-run-events.yaml
        kubectl apply --selector messaging.cloud.google.com/crd-install=true \
        --filename https://github.com/google/knative-gcp/releases/download/{{< version >}}/cloud-run-events.yaml
        kubectl apply --selector events.cloud.google.com/crd-install=true \
        --filename https://github.com/google/knative-gcp/releases/download/{{< version >}}/cloud-run-events.yaml
        ```

     1. To complete the install run the `kubectl apply` command again, this time
        without the `--selector` flags:

        ```shell
        kubectl apply --filename https://github.com/google/knative-gcp/releases/download/{{< version >}}/cloud-run-events.yaml
        ```

1. Configure the authentication mechanism used for accessing the Google Cloud
   services. Currently, we support two methods (Workload Identity and Kubernetes
   Secret). You can choose to apply the provided initialization scripts to ease
   the configuration process or follow the manual steps to have a better
   configuration control.

   1. Initialization Scripts.

      Before applying initialization scripts, make sure your default zone is set
      to be the same as your current cluster. You may use
      `gcloud container clusters describe $CLUSTER_NAME` to get zone and apply
      `gcloud config set compute/zone $ZONE` to set it.

      - Use **Workload Identity**.

        Workload Identity is the recommended way to access Google Cloud services
        from within GKE due to its improved security properties and
        manageability. For more information about Workload Identity, please see
        [here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

        In order to make controller compatible with Workload Identity, use
        [ko](http://github.com/google/ko) to apply
        [controller-gke](../../config/core/deployments/controller-gke.yaml)
        first.

        ```shell
        ko apply -f config/core/deployments/controller-gke.yaml
        ```

        Then you can apply
        [init_control_plane_gke](../../hack/init_control_plane_gke.sh) to
        install all the configuration by running:

        ```shell
        ./hack/init_control_plane_gke.sh
        ```

      - Export service account keys and store them as **Kubernetes Secrets**.
        Apply [init_control_plane](../../hack/init_control_plane.sh) to install
        all the configuration by running:

        ```shell
        ./hack/init_control_plane.sh
        ```

      - **_Note_**: Both scripts will have a step to create a Google Cloud
        Service Account `cloud-run-events`. Ignore the error message if you
        already had this service account (error for 'service account already
        exists').

   1. Manual configuration steps.

      For both methods (Workload Identity and Kubernetes Secret), the first
      manual step is creating a
      [Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project)
      with the appropriate permissions needed for the control plane to manage
      native GCP resources.

      You need to create a new Google Cloud Service Account named
      `cloud-run-events` with the following command:

      ```shell
      gcloud iam service-accounts create cloud-run-events
      ```

      Then, give that Google Cloud Service Account permissions on your project.
      The actual permissions needed will depend on the resources you are
      planning to use. The Table below enumerates such permissions:

      |    Resource / Functionality     |                                     Roles                                      |
      | :-----------------------------: | :----------------------------------------------------------------------------: |
      |        CloudPubSubSource        |                              roles/pubsub.editor                               |
      |       CloudStorageSource        |                              roles/storage.admin                               |
      |      CloudSchedulerSource       |                           roles/cloudscheduler.admin                           |
      |      CloudAuditLogsSource       | roles/pubsub.admin, roles/logging.configWriter, roles/logging.privateLogViewer |
      |        CloudBuildSource         |                            roles/pubsub.subscriber                             |
      |             Channel             |                              roles/pubsub.editor                               |
      |        PullSubscription         |                              roles/pubsub.editor                               |
      |              Topic              |                              roles/pubsub.editor                               |
      | Workload Identity in Data Plane |                         roles/iam.serviceAccountAdmin                          |

      In this guide, and for the sake of simplicity, we will just grant
      `roles/owner` privileges to the Google Cloud Service Account, which
      encompasses all of the above plus some other permissions. Note that if you
      prefer finer-grained privileges, you can just grant the ones described in
      the Table. Also, you can refer to
      [managing multiple projects](../install/managing-multiple-projects.md) in
      case you want your Google Cloud Service Account to manage multiple
      projects.

      ```shell
      gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member=serviceAccount:cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
        --role roles/owner
      ```

      - Use **Workload Identity**.

        Workload Identity is the recommended way to access Google Cloud services
        from within GKE due to its improved security properties and
        manageability. For more information about Workload Identity, please see
        [here](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

        1. Enable Workload Identity.

           ```shell
           gcloud services enable iamcredentials.googleapis.com
           gcloud beta container clusters update $CLUSTER_NAME \
           --identity-namespace=$PROJECT_ID.svc.id.goog
           ```

        1. Bind the Kubernetes Service Account `controller` with Google Cloud
           Service Account.

           ```shell
           MEMBER=serviceAccount:$PROJECT_ID.svc.id.goog[cloud-run-events/controller]

           gcloud iam service-accounts add-iam-policy-binding \
           --role roles/iam.workloadIdentityUser \
           --member $MEMBER cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com
           ```

        1. Add annotation to Kubernetes Service Account `controller`.

           ```shell
           kubectl annotate serviceaccount controller iam.gke.io/gcp-service-account=cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
           --namespace cloud-run-events
           ```

      - Export service account keys and store them as **Kubernetes Secrets**.

        1. Download a new JSON private key for that Service Account. **Be sure
           not to check this key into source control!**

           ```shell
           gcloud iam service-accounts keys create cloud-run-events.json \
           --iam-account=cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com
           ```

        1. Create a Secret on the Kubernetes cluster in the `cloud-run-events`
           namespace with the downloaded key:

           ```shell
           kubectl --namespace cloud-run-events create secret generic google-cloud-key --from-file=key.json=cloud-run-events.json
           ```

           Note that `google-cloud-key` and `key.json` are default values
           expected by our control plane.
