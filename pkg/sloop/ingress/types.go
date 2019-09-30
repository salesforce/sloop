/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package ingress

import (
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
)

type KubeResourceSource interface {
	Init() (chan typed.KubeWatchResult, error)
	Stop()
}

type KubePlaybackFile struct {
	Data []typed.KubeWatchResult `json:"Data"`
}
