/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package kubeextractor

import (
	"hash/fnv"

	"github.com/Jeffail/gabs/v2"
	"github.com/pkg/errors"
)

// VolatileFields are JSON paths stripped before hashing for dedup.
// metadata volatile fields change on every API server touch but don't represent real changes.
// The entire "status" block is stripped because status changes constantly (replicas, metrics,
// container states) but spec/labels/annotations changes are what matter for debugging.
// Full payloads (including status) are still written on every actual write and every 30m snapshot.
var VolatileFields = [][]string{
	{"metadata", "resourceVersion"},
	{"metadata", "generation"},
	{"metadata", "uid"},
	{"metadata", "selfLink"},
	{"metadata", "managedFields"},
	{"status"},
}

// ComputePayloadHash computes an FNV-64a hash of the payload after stripping volatile fields.
// Returns uint64 hash suitable for fast in-memory comparison.
func ComputePayloadHash(payload string) (uint64, error) {
	cleanPayload, err := stripVolatileFields(payload)
	if err != nil {
		return 0, err
	}

	h := fnv.New64a()
	_, err = h.Write([]byte(cleanPayload))
	if err != nil {
		return 0, errors.Wrap(err, "failed to hash payload")
	}

	return h.Sum64(), nil
}

// stripVolatileFields removes known volatile/noisy fields from a Kubernetes resource JSON
// This ensures that the hash is stable across frequent updates that don't represent real changes
func stripVolatileFields(payload string) (string, error) {
	jsonParsed, err := gabs.ParseJSON([]byte(payload))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse JSON payload")
	}

	// Strip each volatile field path, ignoring errors if the path doesn't exist
	for _, fieldPath := range VolatileFields {
		_ = jsonParsed.Delete(fieldPath...)
	}

	return jsonParsed.String(), nil
}
