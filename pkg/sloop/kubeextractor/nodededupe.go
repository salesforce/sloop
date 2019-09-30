/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package kubeextractor

import (
	"fmt"
	"github.com/Jeffail/gabs/v2"
	"github.com/pkg/errors"
)

// In Kubernetes the node objects generally update frequently with new timestamps and resourceVersion
// Because nodes are large resources, it can be desirable to drop updates without important state change
func NodeHasMajorUpdate(node1 string, node2 string) (bool, error) {
	cleanNode1, err := removeResVerAndTimestamp(node1)
	if err != nil {
		return false, err
	}
	cleanNode2, err := removeResVerAndTimestamp(node2)
	if err != nil {
		return false, err
	}
	return !(cleanNode1 == cleanNode2), nil
}

func removeResVerAndTimestamp(nodeJson string) (string, error) {
	jsonParsed, err := gabs.ParseJSON([]byte(nodeJson))
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse json for node resource")
	}

	_, err = jsonParsed.Set("removed", "metadata", "resourceVersion")
	if err != nil {
		return "", errors.Wrap(err, "Could not replace metadata.resourceVersion in node resource")
	}

	numConditions := len(jsonParsed.S("status", "conditions").Children())

	for idx := 0; idx < numConditions; idx += 1 {
		_, err = jsonParsed.Set("removed", "status", "conditions", fmt.Sprint(idx), "lastHeartbeatTime")
		if err != nil {
			return "", errors.Wrap(err, "Could not set node condition")
		}
	}

	return jsonParsed.String(), nil
}
