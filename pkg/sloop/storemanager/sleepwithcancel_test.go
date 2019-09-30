/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package storemanager

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_SleepWithCancel_TestThatSleepsAfterCancelDontCrash(t *testing.T) {
	before := time.Now()
	s := NewSleepWithCancel()
	s.Cancel()
	s.Sleep(time.Minute)
	s.Sleep(time.Minute)
	s.Sleep(time.Hour)
	assert.True(t, time.Since(before).Seconds() < 100)
}
