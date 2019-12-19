/*
Copyright 2019 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"encoding/json"
	"errors"
	"strconv"
)

func Base64ToMap(base64 string) (map[string]string, error) {
	if base64 == "" {
		return nil, errors.New("base64 map string is empty")
	}

	quotedBase64 := strconv.Quote(string(base64))

	var byteExtensions []byte
	err := json.Unmarshal([]byte(quotedBase64), &byteExtensions)
	if err != nil {
		return nil, err
	}

	var extensions map[string]string
	err = json.Unmarshal(byteExtensions, &extensions)
	if err != nil {
		return nil, err
	}

	return extensions, err
}

func MapToBase64(extensions map[string]string) (string, error) {
	if extensions == nil {
		return "", errors.New("map is nil")
	}

	jsonExtensions, err := json.Marshal(extensions)
	if err != nil {
		return "", err
	}
	// if we json.Marshal a []byte, we will get back a base64 encoded quoted string.
	base64Extensions, err := json.Marshal(jsonExtensions)
	if err != nil {
		return "", err
	}

	extensionsString, err := strconv.Unquote(string(base64Extensions))
	if err != nil {
		return "", err
	}
	// Turn the base64 encoded []byte back into a string.
	return extensionsString, nil
}
