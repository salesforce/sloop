/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

// Parameters are shared between webserver and here
// Keep this in sync with pkg/sloop/webfiles/filter.js
const (
	LookbackParam  = "lookback"
	NamespaceParam = "namespace"
	KindParam      = "kind"
	NameParam      = "name"
	NameMatchParam = "namematch" // substring match on name
	UuidParam      = "uuid"
	StartTimeParam = "start_time"
	EndTimeParam   = "end_time"
	ClickTimeParam = "click_time"
	QueryParam     = "query"
	SortParam      = "sort"
)

const (
	AllKinds         = "_all"
	AllNamespaces    = "_all"
	DefaultNamespace = "default"
)
