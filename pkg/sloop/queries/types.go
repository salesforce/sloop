/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package queries

type TimelineRoot struct {
	ViewOpt ViewOptions   `json:"view_options"`
	Rows    []TimelineRow `json:"rows"`
}

type TimelineRow struct {
	Text       string    `json:"text"`
	Duration   int64     `json:"duration"`
	Kind       string    `json:"kind"`
	Namespace  string    `json:"namespace"`
	Overlays   []Overlay `json:"overlays"`
	ChangedAt  []int64   `json:"changedat"`
	NoChangeAt []int64   `json:"nochangeat"`
	StartDate  int64     `json:"start_date"`
	EndDate    int64     `json:"end_date"`
}

type ViewOptions struct {
	Sort string `json:"sort"`
}

type Overlay struct {
	Text      string `json:"text"`
	StartDate int64  `json:"start_date"`
	Duration  int64  `json:"duration"`
	EndDate   int64  `json:"end_date"`
}
