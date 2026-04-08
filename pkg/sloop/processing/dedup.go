/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
)

// DedupEntry holds state for a single resource's deduplication
type DedupEntry struct {
	payloadHash  uint64
	lastWriteAt  time.Time
	resourceType string
	namespace    string
	name         string
}

// DedupState manages in-memory deduplication state for all resources.
// It decides whether incoming watch events should be written to the watch table.
type DedupState struct {
	mu                  sync.RWMutex
	entries             map[string]*DedupEntry
	snapshotInterval    time.Duration
	enabled             bool
	skippedCount        int64
	snapshotCount       int64
	changedCount        int64
	addedCount          int64
	deletedCount        int64
}


// NewDedupState creates a new deduplication state manager
func NewDedupState(enabled bool, snapshotInterval time.Duration) *DedupState {
	return &DedupState{
		entries:          make(map[string]*DedupEntry),
		snapshotInterval: snapshotInterval,
		enabled:          enabled,
	}
}

// resourceKey generates a unique key for a resource
func (ds *DedupState) resourceKey(resourceType, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", resourceType, namespace, name)
}

// ShouldWrite determines if an event should be written to the watch table.
// Returns (shouldWrite bool, reason string) for compatibility with watch.go
func (ds *DedupState) ShouldWrite(namespace, resourceType, name string, payloadHash uint64, eventTime time.Time, snapshotInterval time.Duration) (bool, string) {
	if !ds.enabled {
		return true, "dedup_disabled"
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()

	key := ds.resourceKey(resourceType, namespace, name)

	// For UPDATE events, check if the payload has changed
	entry, exists := ds.entries[key]
	if !exists {
		// First time seeing this resource
		ds.entries[key] = &DedupEntry{
			payloadHash:  payloadHash,
			lastWriteAt:  eventTime,
			resourceType: resourceType,
			namespace:    namespace,
			name:         name,
		}
		ds.changedCount++
		return true, "changed"
	}

	// Resource has been seen before
	if entry.payloadHash != payloadHash {
		// Payload hash changed, meaningful update
		entry.payloadHash = payloadHash
		entry.lastWriteAt = eventTime
		ds.changedCount++
		glog.V(2).Infof("Dedup: writing %s/%s/%s (hash changed)", resourceType, namespace, name)
		return true, "changed"
	}

	// Hash is the same, check if enough time has passed for a snapshot
	timeSinceLastWrite := eventTime.Sub(entry.lastWriteAt)
	if timeSinceLastWrite >= snapshotInterval {
		entry.lastWriteAt = eventTime
		ds.snapshotCount++
		glog.V(2).Infof("Dedup: writing %s/%s/%s (snapshot, %v since last write)", resourceType, namespace, name, timeSinceLastWrite)
		return true, "snapshot"
	}

	// No change and within snapshot interval, skip it
	ds.skippedCount++
	glog.V(3).Infof("Dedup: skipping %s/%s/%s (hash unchanged, %v since last write)", resourceType, namespace, name, timeSinceLastWrite)
	return false, "skipped"
}

// GetStats returns a snapshot of current dedup statistics
func (ds *DedupState) GetStats() map[string]int64 {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	return map[string]int64{
		"skipped":    ds.skippedCount,
		"snapshot":   ds.snapshotCount,
		"changed":    ds.changedCount,
		"added":      ds.addedCount,
		"deleted":    ds.deletedCount,
		"tracked":    int64(len(ds.entries)),
	}
}

// Reset clears all dedup state (useful for testing)
func (ds *DedupState) Reset() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.entries = make(map[string]*DedupEntry)
	ds.skippedCount = 0
	ds.snapshotCount = 0
	ds.changedCount = 0
	ds.addedCount = 0
	ds.deletedCount = 0
}
