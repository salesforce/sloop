/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package kubeextractor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ExtractMetadata_OutputCorrect(t *testing.T) {
	payload := `{"metadata":
					{
						"name":"name1",
						"namespace":"namespace1",
						"selfLink":"link1",
						"uid":"uid1",
						"resourceVersion":"123",
						"creationTimestamp":"2019-07-12T20:12:12Z",
						"ownerReferences": [
						 {
						   "kind": "Deployment",
						   "name": "deployment1",
						   "uid": "uid0"
						 }]
					}
				}`
	expectedResult := KubeMetadata{
		Name:              "name1",
		Namespace:         "namespace1",
		Uid:               "uid1",
		SelfLink:          "link1",
		ResourceVersion:   "123",
		CreationTimestamp: "2019-07-12T20:12:12Z",
		OwnerReferences:   []KubeMetadataOwnerReference{{Kind: "Deployment", Name: "deployment1", Uid: "uid0"}},
	}
	result, err := ExtractMetadata(payload)
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)
}

func Test_ExtractMetadata_MissingFieldsAreIgnored(t *testing.T) {
	payload := `{"metadata":{"name":"name1","uid":"uid1","resourceVersion":"123","creationTimestamp":"2019-07-12T20:12:12Z"}}`
	expectedResult := KubeMetadata{
		Name:              "name1",
		Namespace:         "",
		Uid:               "uid1",
		SelfLink:          "",
		ResourceVersion:   "123",
		CreationTimestamp: "2019-07-12T20:12:12Z",
	}
	result, err := ExtractMetadata(payload)
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)
}

func Test_ExtractMetadata_InvalidPayload_ReturnsError(t *testing.T) {
	payload := `{"metadata":{"name":"name1","namespace":"namespace1","selfLink":"link1"}`
	result, err := ExtractMetadata(payload)
	assert.NotNil(t, err)
	assert.Equal(t, KubeMetadata{}, result)
}

func Test_ExtractMetadata_PayloadHasAdditionalFields_OutputCorrect(t *testing.T) {
	payload := `{"metadata":{"name":"name1","namespace":"namespace1","selfLink":"link1","uid":"uid1","resourceVersion":"123","creationTimestamp":"2019-07-12T20:12:12Z"},"meta2":{"kind":"Pod","namespace":"namespace2"}}`
	expectedResult := KubeMetadata{
		Name:              "name1",
		Namespace:         "namespace1",
		Uid:               "uid1",
		SelfLink:          "link1",
		ResourceVersion:   "123",
		CreationTimestamp: "2019-07-12T20:12:12Z",
	}
	result, err := ExtractMetadata(payload)
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)
}
