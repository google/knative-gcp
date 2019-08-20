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

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
)

func cmd(cmdLine string) *exec.Cmd {
	cmdSplit := strings.Split(cmdLine, " ")
	cmd := cmdSplit[0]
	args := cmdSplit[1:]
	return exec.Command(cmd, args...)
}

func runCmd(cmdLine string) (string, error) {
	cmd := cmd(cmdLine)

	cmdOut, err := cmd.Output()
	return string(cmdOut), err
}

func KoPublish(pack string) (string, error) {
	out, err := runCmd(fmt.Sprintf("ko publish %s", pack))
	if err != nil {
		return "", err
	}
	return out, nil
}
