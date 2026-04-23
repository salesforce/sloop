/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDedupState_Disabled(t *testing.T) {
	ds := NewDedupState(false, 30*time.Minute)
	now := time.Now()

	shouldWrite, reason := ds.ShouldWrite("default", "Pod", "test", 12345, now, 30*time.Minute)
	assert.Equal(t, true, shouldWrite, "Should always write when dedup disabled")
	assert.Equal(t, "dedup_disabled", reason)

	shouldWrite, reason = ds.ShouldWrite("default", "Pod", "test", 12345, now, 30*time.Minute)
	assert.Equal(t, true, shouldWrite, "Should always write when dedup disabled")
	assert.Equal(t, "dedup_disabled", reason)
}

func TestDedupState_FirstWrite_AlwaysWrites(t *testing.T) {
	ds := NewDedupState(true, 30*time.Minute)
	now := time.Now()

	shouldWrite, reason := ds.ShouldWrite("default", "Pod", "test", 12345, now, 30*time.Minute)
	assert.Equal(t, true, shouldWrite)
	assert.Equal(t, "changed", reason)

	stats := ds.GetStats()
	assert.Equal(t, int64(1), stats["changed"])
}

func TestDedupState_HashChanged_Writes(t *testing.T) {
	ds := NewDedupState(true, 30*time.Minute)
	now := time.Now()
	interval := 30 * time.Minute

	// First event
	shouldWrite, reason := ds.ShouldWrite("default", "Pod", "test", 11111, now, interval)
	assert.Equal(t, true, shouldWrite)
	assert.Equal(t, "changed", reason)

	// Same hash 1 minute later
	shouldWrite, reason = ds.ShouldWrite("default", "Pod", "test", 11111, now.Add(1*time.Minute), interval)
	assert.Equal(t, false, shouldWrite)
	assert.Equal(t, "skipped", reason)

	// Event with different hash
	shouldWrite, reason = ds.ShouldWrite("default", "Pod", "test", 22222, now.Add(2*time.Minute), interval)
	assert.Equal(t, true, shouldWrite)
	assert.Equal(t, "changed", reason)

	stats := ds.GetStats()
	assert.Equal(t, int64(1), stats["skipped"])
	assert.Equal(t, int64(2), stats["changed"])
}

func TestDedupState_SnapshotInterval_WritesAfterInterval(t *testing.T) {
	interval := 30 * time.Minute
	ds := NewDedupState(true, interval)
	now := time.Now()

	// First event
	shouldWrite, reason := ds.ShouldWrite("default", "Pod", "test", 11111, now, interval)
	assert.Equal(t, true, shouldWrite)
	assert.Equal(t, "changed", reason)

	// Same hash at 15 minutes
	shouldWrite, reason = ds.ShouldWrite("default", "Pod", "test", 11111, now.Add(15*time.Minute), interval)
	assert.Equal(t, false, shouldWrite)
	assert.Equal(t, "skipped", reason)

	// Same hash at 31 minutes (beyond interval)
	shouldWrite, reason = ds.ShouldWrite("default", "Pod", "test", 11111, now.Add(31*time.Minute), interval)
	assert.Equal(t, true, shouldWrite)
	assert.Equal(t, "snapshot", reason)

	// Same hash at 32 minutes (just after interval)
	shouldWrite, reason = ds.ShouldWrite("default", "Pod", "test", 11111, now.Add(32*time.Minute), interval)
	assert.Equal(t, false, shouldWrite)
	assert.Equal(t, "skipped", reason)

	// Same hash at 62 minutes (another interval)
	shouldWrite, reason = ds.ShouldWrite("default", "Pod", "test", 11111, now.Add(62*time.Minute), interval)
	assert.Equal(t, true, shouldWrite)
	assert.Equal(t, "snapshot", reason)

	stats := ds.GetStats()
	assert.Equal(t, int64(2), stats["skipped"])
	assert.Equal(t, int64(2), stats["snapshot"])
	assert.Equal(t, int64(1), stats["changed"])
}

func TestDedupState_MultipleResources_Tracked(t *testing.T) {
	ds := NewDedupState(true, 30*time.Minute)
	now := time.Now()
	interval := 30 * time.Minute

	// Add 3 different resources
	ds.ShouldWrite("default", "Pod", "pod1", 11111, now, interval)
	ds.ShouldWrite("default", "Pod", "pod2", 22222, now, interval)
	ds.ShouldWrite("kube-system", "Deployment", "coredns", 33333, now, interval)

	stats := ds.GetStats()
	assert.Equal(t, int64(3), stats["tracked"])
	assert.Equal(t, int64(3), stats["changed"])
}

func TestDedupState_DifferentNamespacesSeparate(t *testing.T) {
	ds := NewDedupState(true, 30*time.Minute)
	now := time.Now()
	interval := 30 * time.Minute

	// Same name, different namespace
	ds.ShouldWrite("default", "Pod", "test", 11111, now, interval)
	shouldWrite, _ := ds.ShouldWrite("kube-system", "Pod", "test", 11111, now, interval)

	// Should be treated as different resources
	assert.Equal(t, true, shouldWrite)

	stats := ds.GetStats()
	assert.Equal(t, int64(2), stats["tracked"])
}

func TestDedupState_DifferentKindsSeparate(t *testing.T) {
	ds := NewDedupState(true, 30*time.Minute)
	now := time.Now()
	interval := 30 * time.Minute

	// Same name and namespace, different kind
	ds.ShouldWrite("default", "Pod", "test", 11111, now, interval)
	shouldWrite, _ := ds.ShouldWrite("default", "Service", "test", 11111, now, interval)

	// Should be treated as different resources
	assert.Equal(t, true, shouldWrite)

	stats := ds.GetStats()
	assert.Equal(t, int64(2), stats["tracked"])
}

func TestDedupState_FirstEventIsWrite(t *testing.T) {
	ds := NewDedupState(true, 30*time.Minute)
	now := time.Now()

	shouldWrite, reason := ds.ShouldWrite("default", "Pod", "new-resource", 99999, now, 30*time.Minute)
	assert.Equal(t, true, shouldWrite, "First UPDATE for a resource should always write")
	assert.Equal(t, "changed", reason)

	stats := ds.GetStats()
	assert.Equal(t, int64(1), stats["changed"])
	assert.Equal(t, int64(1), stats["tracked"])
}

func TestDedupState_Reset(t *testing.T) {
	ds := NewDedupState(true, 30*time.Minute)
	now := time.Now()
	interval := 30 * time.Minute

	ds.ShouldWrite("default", "Pod", "test", 11111, now, interval)
	stats1 := ds.GetStats()
	assert.Equal(t, int64(1), stats1["tracked"])

	ds.Reset()
	stats2 := ds.GetStats()
	assert.Equal(t, int64(0), stats2["tracked"])
	assert.Equal(t, int64(0), stats2["changed"])
}

func TestDedupState_GetStats(t *testing.T) {
	ds := NewDedupState(true, 30*time.Minute)
	now := time.Now()
	interval := 30 * time.Minute

	ds.ShouldWrite("default", "Pod", "test1", 11111, now, interval)
	ds.ShouldWrite("default", "Pod", "test1", 11111, now.Add(1*time.Minute), interval)
	ds.ShouldWrite("default", "Pod", "test1", 11111, now.Add(2*time.Minute), interval)
	ds.ShouldWrite("default", "Pod", "test1", 22222, now.Add(3*time.Minute), interval)

	stats := ds.GetStats()
	assert.Equal(t, int64(2), stats["skipped"])
	assert.Equal(t, int64(2), stats["changed"])
	assert.Equal(t, int64(1), stats["tracked"])
}
