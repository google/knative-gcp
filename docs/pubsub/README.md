# Enable Cloud Pub/Sub

The following are prerequisites for `Channel`, `Topic` and `PullSubscription`.

## Installing Pub/Sub Enabled Service Account

1.  Create a
    [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
    and install the `gcloud` CLI and run `gcloud auth login`. This sample will
    use a mix of `gcloud` and `kubectl` commands. The rest of the sample assumes
    that you've set the `$PROJECT_ID` environment variable to your Google Cloud
    project id, and also set your project ID as default using
    `gcloud config set project $PROJECT_ID`.

1.  Enable the `Cloud Pub/Sub API` on your project:

    ```shell
    gcloud services enable pubsub.googleapis.com
    ```

1.  Create a
    [Google Cloud Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project).
    This sample creates one service account for both registration and receiving
    messages, but you can also create a separate service account for receiving
    messages if you want additional privilege separation.

    1.  Create a new service account named `cloudrunevents-pullsub` with the
        following command:

        ```shell
        gcloud iam service-accounts create cloudrunevents-pullsub
        ```

    1.  Give that Service Account the `Pub/Sub Editor` role on your Google Cloud
        project:

        ```shell
        gcloud projects add-iam-policy-binding $PROJECT_ID \
          --member=serviceAccount:cloudrunevents-pullsub@$PROJECT_ID.iam.gserviceaccount.com \
          --role roles/pubsub.editor
        ```

    1.  **Optional:** If you plan on using the StackDriver monitoring APIs, also
        give the Service Account the `Monitoring MetricWriter` role on your
        Google Cloud project:

        ```shell
        gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member=serviceAccount:cloudrunevents-pullsub@$PROJECT_ID.iam.gserviceaccount.com \
        --role roles/monitoring.metricWriter
        ```

    1.  Download a new JSON private key for that Service Account. **Be sure not
        to check this key into source control!**

        ```shell
        gcloud iam service-accounts keys create cloudrunevents-pullsub.json \
        --iam-account=cloudrunevents-pullsub@$PROJECT_ID.iam.gserviceaccount.com
        ```

    1.  Create a secret on the kubernetes cluster with the downloaded key:

        ```shell
        # The secret should not already exist, so just try to create it.
        kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=cloudrunevents-pullsub.json
        ```

        `google-cloud-key` and `key.json` are default values expected by
        `Channel`, `Topic` and `PullSubscription`.
