/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package kubeextractor

import (
	"encoding/json"
)

type KubeMetadata struct {
	Name              string
	Namespace         string
	Uid               string
	SelfLink          string
	ResourceVersion   string
	CreationTimestamp string
	OwnerReferences   []KubeMetadataOwnerReference
}

type KubeInvolvedObject struct {
	Kind      string
	Name      string
	Namespace string
	Uid       string
}

type KubeMetadataOwnerReference struct {
	Kind string
	Name string
	Uid  string
}

// Extracts metadata from kube watch event payload.
func ExtractMetadata(payload string) (KubeMetadata, error) {
	resource := struct {
		Metadata KubeMetadata
	}{}
	err := json.Unmarshal([]byte(payload), &resource)
	if err != nil {
		return KubeMetadata{}, err
	}
	return resource.Metadata, nil
}
