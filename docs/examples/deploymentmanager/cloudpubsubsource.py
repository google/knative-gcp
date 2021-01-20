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


def GenerateConfig(context):
    """Create a Knative Trigger."""

    propertiesWithoutDmMeta = context.properties.copy()
    del propertiesWithoutDmMeta['dmMeta']
    propertiesWithoutDmMeta['apiVersion'] = 'events.cloud.google.com/v1'
    propertiesWithoutDmMeta['kind'] = 'CloudPubSubSource'
    if 'name' not in propertiesWithoutDmMeta['metadata']:
        propertiesWithoutDmMeta['metadata']['name'] = context.env['name']
    source = {
        'name': context.env['name'],
        'type': ''.join([context.env['project'], '/', context.properties['dmMeta']['typeProvider'], ':',
                         '/apis/events.cloud.google.com/v1/namespaces/{namespace}/cloudpubsubsources/{name}']),
        'properties': propertiesWithoutDmMeta,
    }

    if 'dependsOn' in context.properties['dmMeta']:
        source['metadata'] = {
            'dependsOn': context.properties['dmMeta']['dependsOn'],
        }

    return {'resources': [source]}
