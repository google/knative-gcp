# Copyright 2021 Google Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Create configuration to deploy Kubernetes resources."""

clusterCreateStage = '1_clusterCreate'
acquireOperatorStage = '2_acquireOperator'
installEventingStage = '3_installEventing'
initializeEventing = '4_initializeEventing'


def GenerateConfig(context):
    """Activate Knative-GCP on a given GKE cluster using workload-identity-gsa.

    This does the following:
    1. Enables the required APIs on the project (optional).
    2. Installs eventing via the CloudRun add-on operator.
    3. Creates the three GSAs.
        - Includes giving the Controller GSA service account admin on the Sources GSA.
        - Includes binding the KSAs to the Controller and Broker data plane GSAs.
    4. Gives the three GSAs the required permissions on the project (optional).
    5. Binds the Controller and Broker data plane KSAs to their GSAs.
    6. Sets the config-gcp-auth ConfigMap.
    """
    operatorType = ''.join([context.env['project'], '/', context.properties['typeProvider'], ':',
                            '/apis/operator.run.cloud.google.com/v1alpha1/namespaces/{namespace}/cloudruns/{name}'])
    ksaType = ''.join([context.env['project'], '/', context.properties['typeProvider'], ':',
                       '/api/v1/namespaces/{namespace}/serviceaccounts/{name}'])

    resources = []

    if context.properties['stage'] == clusterCreateStage:
        # We want to give the operator a bit of time to get ready.
        return {'resources': resources}

    # 1. Enables the required APIs on the project (optional).
    if context.properties['activateAPIsOnProject']:
        resources.extend(activateAPIResources(context, [
            'cloudresourcemanager.googleapis.com',
            'cloudscheduler.googleapis.com',
            'logging.googleapis.com',
            'pubsub.googleapis.com',
            'stackdriver.googleapis.com',
            'storage-api.googleapis.com',
            'storage-component.googleapis.com',
        ]))

    operatorEnableEventingName = 'cloud-run'

    dependsOnTypeProvider = []
    if context.properties['dependsOnTypeProvider']:
        dependsOnTypeProvider = [
            context.properties['typeProvider'],
        ]

    dependsOnTypeProviderAndOperatorEnable = dependsOnTypeProvider.copy()
    dependsOnTypeProviderAndOperatorEnable.append(operatorEnableEventingName)

    # 2. Installs eventing via the CloudRun add-on operator.
    eventingEnabled = True
    if context.properties['stage'] == acquireOperatorStage:
        # We will only acquire the operator during clusterCreate, so make sure it has a different spec when it will be updated.
        eventingEnabled = False

    resources.append({
        'name': operatorEnableEventingName,
        'type': operatorType,
        'metadata': {
            'dependsOn': dependsOnTypeProvider,
            'createPolicy': 'ACQUIRE',
        },
        'properties': {
            'apiVersion': 'operator.run.cloud.google.com/v1alpha1',
            'kind': 'CloudRun',
            'metadata': {
                'namespace': 'cloud-run-system',
                'name': 'cloud-run',
            },
            'spec': {
                'eventing': {
                    'enabled': eventingEnabled,
                },
            },
        },
    })

    if context.properties['stage'] == acquireOperatorStage:
        # We just want to acquire the Operator.
        return {'resources': resources}

    # 3. Creates the three GSAs.
    #     - Includes giving the Controller GSA service account admin on the Sources GSA.
    #     - Includes binding the KSAs to the Controller and Broker data plane GSAs.
    resources.append({
        'name': context.properties['controllerGSA'],
        'type': 'iam.v1.serviceAccount',
        'accessControl': {
            'gcpIamPolicy': {
                'bindings': [
                    {
                        'role': 'roles/iam.workloadIdentityUser',
                        'members': [
                            ''.join(['serviceAccount:', context.env['project'],
                                     '.svc.id.goog[cloud-run-events/controller]'])
                        ],
                    },
                ],
            },
        },
        'properties': {
            'accountId': context.properties['controllerGSA'],
        },
    })
    resources.append({
        'name': context.properties['brokerGSA'],
        'type': 'iam.v1.serviceAccount',
        'accessControl': {
            'gcpIamPolicy': {
                'bindings': [
                    {
                        'role': 'roles/iam.workloadIdentityUser',
                        'members': [
                            ''.join(['serviceAccount:', context.env['project'],
                                     '.svc.id.goog[cloud-run-events/broker]'])
                        ],
                    },
                ],
            },
        },
        'properties': {
            'accountId': context.properties['brokerGSA'],
        },
    })
    resources.append({
        'name': context.properties['sourcesGSA'],
        'type': 'iam.v1.serviceAccount',
        'accessControl': {
            'gcpIamPolicy': {
                'bindings': [
                    {
                        'role': 'roles/iam.serviceAccountAdmin',
                        'members': [
                            ''.join(
                                ['serviceAccount:$(ref.', context.properties['controllerGSA'], '.email)'])
                        ],
                    },
                ],
            },
        },
        'properties': {
            'accountId': context.properties['sourcesGSA'],
        },
    })

    # 4. Gives the three GSAs the required permissions on the project (optional).
    if context.properties['grantGSAsPermissionsOnProject']:
        resources.append({
            'name': 'controller-gsa-permissions',
            'type': 'gcp-types/cloudresourcemanager-v1:virtual.projects.iamMemberBinding',
            'properties': {
                'resource': context.env['project'],
                'role': 'roles/kuberun.eventsControlPlaneServiceAgent',
                'member': ''.join(['serviceAccount:$(ref.', context.properties['controllerGSA'], '.email)']),
            },
        })
        resources.append({
            'name': 'broker-gsa-permissions',
            'type': 'gcp-types/cloudresourcemanager-v1:virtual.projects.iamMemberBinding',
            'properties': {
                'resource': context.env['project'],
                'role': 'roles/kuberun.eventsDataPlaneServiceAgent',
                'member': ''.join(['serviceAccount:$(ref.', context.properties['brokerGSA'], '.email)']),
            },
        })
        resources.append({
            'name': 'sources-gsa-permissions',
            'type': 'gcp-types/cloudresourcemanager-v1:virtual.projects.iamMemberBinding',
            'properties': {
                'resource': context.env['project'],
                'role': 'roles/kuberun.eventsDataPlaneServiceAgent',
                'member': ''.join(['serviceAccount:$(ref.', context.properties['sourcesGSA'], '.email)']),
            },
        })

    # 5. Binds the Controller and Broker data plane KSAs to their GSAs.
    controllerKsaAnnotations = {
        'iam.gke.io/gcp-service-account': ''.join([context.properties['controllerGSA'], '@', context.env['project'], '.iam.gserviceaccount.com']),
    }
    if context.properties['stage'] == installEventingStage:
        # We need to first acquire, then update the KSAs, so make sure there is a spec difference during the update.
        controllerKsaAnnotations = {}
    resources.append({
        'name': 'control-plane-ksa',
        'type': ksaType,
        'metadata': {
            'dependsOn': dependsOnTypeProviderAndOperatorEnable,
            'createPolicy': 'ACQUIRE',
        },
        'properties': {
            'apiVersion': 'v1',
            'kind': 'ServiceAccount',
            'metadata': {
                'namespace': 'cloud-run-events',
                'name': 'controller',
                'annotations': controllerKsaAnnotations,
            },
        },
    })
    brokerKsaAnnotations = {
        'iam.gke.io/gcp-service-account': ''.join([context.properties['brokerGSA'], '@', context.env['project'], '.iam.gserviceaccount.com']),
    }
    if context.properties['stage'] == installEventingStage:
        # We need to first acquire, then update the KSAs, so make sure there is a spec difference during the update.
        brokerKsaAnnotations = {}
    resources.append({
        'name': 'broker-ksa',
        'type': ksaType,
        'metadata': {
            'dependsOn': dependsOnTypeProviderAndOperatorEnable,
            'createPolicy': 'ACQUIRE',
        },
        'properties': {
            'apiVersion': 'v1',
            'kind': 'ServiceAccount',
            'metadata': {
                'namespace': 'cloud-run-events',
                'name': 'broker',
                'annotations': brokerKsaAnnotations,
            },
        },
    })

    # 6. Sets the config-gcp-auth ConfigMap.
    configGcpAuthAnnotations = {
        'events.cloud.google.com/initialized': 'true',
    }
    if context.properties['stage'] == installEventingStage:
        # We need to first acquire, then update the ConfigMap, so make sure there is a spec difference during the update.
        configGcpAuthAnnotations = {}
    resources.append({
        'name': 'config-gcp-auth',
        'type': 'config-map.py',
        'properties': {
            'dmMeta': {
                'typeProvider': context.properties['typeProvider'],
                'dependsOn': dependsOnTypeProviderAndOperatorEnable,
                'createPolicy': 'ACQUIRE',
            },
            'metadata': {
                'namespace': 'cloud-run-events',
                'name': 'config-gcp-auth',
                'annotations': configGcpAuthAnnotations,
            },
            'data': {
                'default-auth-config': ''.join([
                    'clusterDefault:\n'
                    '  serviceAccountName: cloud-run-events-sources\n'
                    '  workloadIdentityMapping:\n'
                    '    cloud-run-events-sources: ', context.properties[
                        'sourcesGSA'], '@', context.env['project'], '.iam.gserviceaccount.com\n'
                ])
            },
        },
    })

    return {'resources': resources}


def activateAPIResources(context, apis):
    resources = []
    for index, api in apis:
        depends_on = []
        # Serialize the activation of all the apis by making api_n depend on api_n-1
        if (not context.properties['concurrentAPIActivation']) and index != 0:
            depends_on.append(
                ApiResourceName(context.env['project'], context.properties['apis'][index-1]))
        resources.append({
            'name': ApiResourceName(context.env['project'], api),
            'type': 'deploymentmanager.v2.virtual.enableService',
            'metadata': {
                'dependsOn': depends_on
            },
            'properties': {
                'consumerId': 'project:' + context.env['project'],
                'serviceName': api
            }
        })
    return resources


def ApiResourceName(project_id, api_name):
    return project_id + '-' + api_name
