/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package kubeextractor

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

const nodeTemplate = `{
  "metadata": {
    "name": "somehostname",
    "uid": "1f9c4fdc-df86-11e6-8ec4-141877585f71",
    "resourceVersion": "{{.ResourceVersion}}"
  },
  "status": {
    "conditions": [
      {
        "type": "OutOfDisk",
        "status": "{{.OutOfDisk}}",
        "lastHeartbeatTime": "{{.LastHeartbeatTime}}",
        "lastTransitionTime": "2019-07-19T15:35:56Z",
        "reason": "KubeletHasSufficientDisk"
      },
      {
        "type": "MemoryPressure",
        "status": "False",
        "lastHeartbeatTime": "{{.LastHeartbeatTime}}",
        "lastTransitionTime": "2019-07-19T15:35:56Z",
        "reason": "KubeletHasSufficientMemory"
      }
    ]
  }
}
`

const someResourceVersion1 = "873691308"
const someResourceVersion2 = "873691358"
const someHeartbeatTime1 = "2019-07-23T17:18:10Z"
const someHeartbeatTime2 = "2019-07-23T17:18:20Z"

type nodeData struct {
	ResourceVersion   string
	LastHeartbeatTime string
	OutOfDisk         string
}

func helper_makeNodeResource(t *testing.T, resVer string, heartbeat string, outOfDisk string) string {
	data := nodeData{ResourceVersion: resVer, LastHeartbeatTime: heartbeat, OutOfDisk: outOfDisk}
	tmp, err := template.New("test").Parse(nodeTemplate)
	assert.Nil(t, err)
	var tpl bytes.Buffer
	err = tmp.Execute(&tpl, data)
	assert.Nil(t, err)
	return tpl.String()
}

const expectedCleanNode = `{"metadata":{"name":"somehostname","resourceVersion":"removed","uid":"1f9c4fdc-df86-11e6-8ec4-141877585f71"},"status":{"conditions":[` +
	`{"lastHeartbeatTime":"removed","lastTransitionTime":"2019-07-19T15:35:56Z","reason":"KubeletHasSufficientDisk","status":"False","type":"OutOfDisk"},` +
	`{"lastHeartbeatTime":"removed","lastTransitionTime":"2019-07-19T15:35:56Z","reason":"KubeletHasSufficientMemory","status":"False","type":"MemoryPressure"}]}}`

func Test_removeResVerAndTimestamp(t *testing.T) {
	nodeJson := helper_makeNodeResource(t, someResourceVersion1, someHeartbeatTime1, "False")
	cleanNode, err := removeResVerAndTimestamp(nodeJson)
	assert.Nil(t, err)
	fmt.Printf("%v\n", cleanNode)
	assert.Equal(t, expectedCleanNode, cleanNode)
}

func Test_nodesMeaningfullyDifferent_sameNode(t *testing.T) {
	nodeJson := helper_makeNodeResource(t, someResourceVersion1, someHeartbeatTime1, "False")
	diff, err := NodeHasMajorUpdate(nodeJson, nodeJson)
	assert.Nil(t, err)
	assert.False(t, diff)
}

func Test_nodesMeaningfullyDifferent_onlyDiffTimeAndRes(t *testing.T) {
	nodeJson1 := helper_makeNodeResource(t, someResourceVersion1, someHeartbeatTime1, "False")
	nodeJson2 := helper_makeNodeResource(t, someResourceVersion2, someHeartbeatTime2, "False")
	diff, err := NodeHasMajorUpdate(nodeJson1, nodeJson2)
	assert.Nil(t, err)
	assert.False(t, diff)
}

func Test_nodesMeaningfullyDifferent_diffOutOfDisk(t *testing.T) {
	nodeJson1 := helper_makeNodeResource(t, someResourceVersion1, someHeartbeatTime1, "False")
	nodeJson2 := helper_makeNodeResource(t, someResourceVersion1, someHeartbeatTime1, "True")
	diff, err := NodeHasMajorUpdate(nodeJson1, nodeJson2)
	assert.Nil(t, err)
	assert.True(t, diff)
}
