/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package storemanager

import (
	"time"
)

// This provides a way to Sleep with the ability to get woken up for a cancel
// Once cancel is called all future sleeps will return immediately

type SleepWithCancel struct {
	cancel chan bool
}

func NewSleepWithCancel() *SleepWithCancel {
	return &SleepWithCancel{cancel: make(chan bool, 10)}
}

func (s *SleepWithCancel) Sleep(after time.Duration) {
	select {
	case <-s.cancel:
		break
	case <-time.After(after):
		break
	}
}

func (s *SleepWithCancel) Cancel() {
	s.cancel <- true
	close(s.cancel)
}
