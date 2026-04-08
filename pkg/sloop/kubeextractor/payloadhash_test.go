/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package kubeextractor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputePayloadHash_StableForSamePayload(t *testing.T) {
	payload := `{
  "metadata": {
    "name": "test-pod",
    "namespace": "default",
    "resourceVersion": "12345"
  },
  "spec": {
    "containers": [
      {
        "name": "app",
        "image": "app:v1"
      }
    ]
  }
}`

	hash1, err := ComputePayloadHash(payload)
	assert.NoError(t, err)

	hash2, err := ComputePayloadHash(payload)
	assert.NoError(t, err)

	assert.Equal(t, hash1, hash2, "Hash should be stable for identical payloads")
}

func TestComputePayloadHash_IgnoresResourceVersion(t *testing.T) {
	payload1 := `{
  "metadata": {
    "name": "test-pod",
    "namespace": "default",
    "resourceVersion": "12345"
  },
  "spec": {
    "containers": [
      {
        "name": "app",
        "image": "app:v1"
      }
    ]
  }
}`

	payload2 := `{
  "metadata": {
    "name": "test-pod",
    "namespace": "default",
    "resourceVersion": "99999"
  },
  "spec": {
    "containers": [
      {
        "name": "app",
        "image": "app:v1"
      }
    ]
  }
}`

	hash1, err := ComputePayloadHash(payload1)
	assert.NoError(t, err)

	hash2, err := ComputePayloadHash(payload2)
	assert.NoError(t, err)

	assert.Equal(t, hash1, hash2, "Hash should ignore resourceVersion changes")
}

func TestComputePayloadHash_IgnoresStatusConditions(t *testing.T) {
	payload1 := `{
  "metadata": {
    "name": "test-pod",
    "namespace": "default"
  },
  "spec": {
    "containers": [
      {
        "name": "app",
        "image": "app:v1"
      }
    ]
  },
  "status": {
    "phase": "Running",
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "lastTransitionTime": "2026-04-01T10:00:00Z"
      }
    ]
  }
}`

	payload2 := `{
  "metadata": {
    "name": "test-pod",
    "namespace": "default"
  },
  "spec": {
    "containers": [
      {
        "name": "app",
        "image": "app:v1"
      }
    ]
  },
  "status": {
    "phase": "Running",
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "lastTransitionTime": "2026-04-02T15:30:00Z"
      }
    ]
  }
}`

	hash1, err := ComputePayloadHash(payload1)
	assert.NoError(t, err)

	hash2, err := ComputePayloadHash(payload2)
	assert.NoError(t, err)

	assert.Equal(t, hash1, hash2, "Hash should ignore status conditions and timestamps")
}

func TestComputePayloadHash_DetectsMeaningfulChanges(t *testing.T) {
	payloadBefore := `{
  "metadata": {
    "name": "test-pod",
    "namespace": "default"
  },
  "spec": {
    "containers": [
      {
        "name": "app",
        "image": "app:v1"
      }
    ]
  }
}`

	payloadAfter := `{
  "metadata": {
    "name": "test-pod",
    "namespace": "default"
  },
  "spec": {
    "containers": [
      {
        "name": "app",
        "image": "app:v2"
      }
    ]
  }
}`

	hashBefore, err := ComputePayloadHash(payloadBefore)
	assert.NoError(t, err)

	hashAfter, err := ComputePayloadHash(payloadAfter)
	assert.NoError(t, err)

	assert.NotEqual(t, hashBefore, hashAfter, "Hash should differ for meaningful spec changes")
}

func TestComputePayloadHash_InvalidJSON(t *testing.T) {
	invalidPayload := `{invalid json`

	_, err := ComputePayloadHash(invalidPayload)
	assert.Error(t, err, "Should error on invalid JSON")
}

func TestStripVolatileFields_RemovesResourceVersion(t *testing.T) {
	payload := `{
  "metadata": {
    "name": "test",
    "resourceVersion": "12345"
  },
  "spec": {
    "image": "img:v1"
  }
}`

	cleaned, err := stripVolatileFields(payload)
	assert.NoError(t, err)
	assert.NotContains(t, cleaned, "12345", "resourceVersion should be removed")
	assert.Contains(t, cleaned, "test", "name should remain")
	assert.Contains(t, cleaned, "img:v1", "spec.image should remain")
}

func TestStripVolatileFields_PreservesSpec(t *testing.T) {
	payload := `{
  "metadata": {
    "name": "test-pod"
  },
  "spec": {
    "replicas": 3,
    "template": {
      "spec": {
        "containers": [
          {
            "name": "app",
            "image": "app:latest"
          }
        ]
      }
    }
  }
}`

	cleaned, err := stripVolatileFields(payload)
	assert.NoError(t, err)
	assert.Contains(t, cleaned, "replicas", "spec.replicas should be preserved")
	assert.Contains(t, cleaned, "app:latest", "container image should be preserved")
}
