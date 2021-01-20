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
    """Create a ConfigMap."""

    propertiesWithoutDmMeta = context.properties.copy()
    del propertiesWithoutDmMeta['dmMeta']
    propertiesWithoutDmMeta['apiVersion'] = 'v1'
    propertiesWithoutDmMeta['kind'] = 'ConfigMap'
    if 'name' not in propertiesWithoutDmMeta['metadata']:
        propertiesWithoutDmMeta['metadata']['name'] = context.env['name']
    cm = {
        'name': context.env['name'],
        'type': ''.join([context.env['project'], '/', context.properties['dmMeta']['typeProvider'], ':',
                         '/api/v1/namespaces/{namespace}/configmaps/{name}']),
        'properties': propertiesWithoutDmMeta,
    }

    if 'dependsOn' in context.properties['dmMeta']:
        cm['metadata'] = {
            'dependsOn': context.properties['dmMeta']['dependsOn'],
        }

    return {'resources': [cm]}
