# Installing Knative with GCP

1. Create a
   [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
   and install the `gcloud` CLI and run `gcloud auth login`. This guide will
   use a mix of `gcloud` and `kubectl` commands. The rest of the guide assumes
   that you've set the `PROJECT_ID` environment variable to your Google Cloud
   project id, and also set your project ID as default using
   `gcloud config set project $PROJECT_ID`.

1. Install [Knative](https://knative.dev/docs/install/). Preferably, set up both [Serving](https://knative.dev/docs/serving/)
   and [Eventing](https://knative.dev/docs/eventing/). The latter is only required if you want to use the Pub/Sub `Channel`.  

1. Create the `cloud-run-events` namespace. This is the namespace where our control plane pods run.

     ```shell
     kubectl create namespace cloud-run-events
     ```

1.  Create a [Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project) with the
    appropriate permissions needed for the control plane to manage native GCP resources.
    
    1. Create a new Service Account named `cloud-run-events` with the following command:
        
        ```shell
        gcloud iam service-accounts create cloud-run-events
        ```

    1. Give that Service Account permissions on your project. The actual permissions needed will depend on the resources you 
       are planning to use. The Table below enumerates such permissions:
       
        |     Resource     	|            Roles           	|
        |:----------------:	|:--------------------------:	|
        |      PubSub      	|     roles/pubsub.editor    	|
        |      Storage     	|     roles/storage.admin    	|
        |     Scheduler    	| roles/cloudscheduler.admin 	|
        |      Channel     	|     roles/pubsub.editor    	|
        | PullSubscription 	|     roles/pubsub.editor    	|
        |       Topic      	|     roles/pubsub.editor    	|
       
       In this guide, and for the sake of simplicity, we will just grant `roles/editor` privileges to the Service Account,
       which encompasses all of the above plus some other permissions. Note that if you prefer finer-grained privileges,
       you can just grant the ones described in the Table. Also, you can refer to [managing multiple projects](../install/managing-multiple-projects.md)
       in case you want your Service Account to manage multiple projects.

        ```shell
        gcloud projects add-iam-policy-binding $PROJECT_ID \
          --member=serviceAccount:cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com \
          --role roles/editor
        ```

    1.  Download a new JSON private key for that Service Account. **Be sure not to check this key into source control!**
    
        ```shell
        gcloud iam service-accounts keys create cloud-run-events.json \
        --iam-account=cloud-run-events@$PROJECT_ID.iam.gserviceaccount.com
        ```

    1.  Create a Secret on the Kubernetes cluster in the `cloud-run-events` namespace with the downloaded key:
    
        ```shell
        kubectl --namespace cloud-run-events create secret generic google-cloud-key --from-file=key.json=cloud-run-events.json
        ```
    
        Note that `google-cloud-key` and `key.json` are default values expected by our control plane.

1. Finally, install the Knative-GCP constructs. You can either:

    - Install from master using [ko](http://github.com/google/ko)
        
        ```shell
        ko apply -f ./config
        ```
    OR
    - Install a [release](https://github.com/google/knative-gcp/releases). Remember to update `{{< version >}}` in the
      commands below with the appropriate release version.

       1. First install the CRDs by running the `kubectl apply`
          command with the `--selector` flag. This prevents race conditions during the install, which cause intermittent errors:
    
            ```shell
            kubectl apply --selector pubsub.cloud.google.com/crd-install=true \
            --filename https://github.com/google/knative-gcp/releases/download/{{< version >}}/cloud-run-events.yaml            
            kubectl apply --selector messaging.cloud.google.com/crd-install=true \
            --filename https://github.com/google/knative-gcp/releases/download/{{< version >}}/cloud-run-events.yaml            
            kubectl apply --selector events.cloud.google.com/crd-install=true \
            --filename https://github.com/google/knative-gcp/releases/download/{{< version >}}/cloud-run-events.yaml
            ```
    
        1. To complete the install run the `kubectl apply` command again, this time without the `--selector` flags:
    
            ```shell
            kubectl apply --filename https://github.com/google/knative-gcp/releases/download/{{< version >}}/cloud-run-events.yaml            
            ```
