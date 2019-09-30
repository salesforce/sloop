/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func timeFromUnixTimeParam(request *http.Request, paramName string, defaultTs time.Time, unit time.Duration) (time.Time, error) {
	unixStr := request.URL.Query().Get(paramName)
	if unixStr == "" {
		return defaultTs, nil
	}
	startDateInt64, err := strconv.ParseInt(unixStr, 10, 64)
	if err != nil {
		return defaultTs, err
	}
	if unit == time.Second {
		return time.Unix(startDateInt64, 0).UTC(), nil
	} else if unit == time.Millisecond {
		return time.Unix(startDateInt64/1000, 1000*1000*(startDateInt64%1000)).UTC(), nil
	} else {
		return time.Time{}, fmt.Errorf("Invalid unit.  Only second and millisecond are supported")
	}
}

func durationFromParam(request *http.Request, paramName string, defaultDuration time.Duration) (time.Duration, error) {
	durStr := request.URL.Query().Get(paramName)
	if durStr == "" {
		return defaultDuration, nil
	}
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return defaultDuration, err
	}
	return dur, nil
}

// Need to allow:
// 1) Kubernetes names: 253 chars, lower a-z, '-' and '.'
// 2) All const "_all"
// 3) Resource names that start with upper "Pod"
func cleanStringFromParam(request *http.Request, paramName string, defaultStr string) string {
	strVal := request.URL.Query().Get(paramName)
	if strVal == "" {
		return defaultStr
	}
	reg := regexp.MustCompile("[^a-zA-Z0-9\\-_.]+")
	clean := reg.ReplaceAllString(strVal, "")
	clean = strings.ReplaceAll(clean, "..", "")
	return clean
}

func numberFromParam(request *http.Request, paramName string, defaultNum int) int {
	numStr := request.URL.Query().Get(paramName)
	if numStr == "" {
		return defaultNum
	}
	numVal, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return defaultNum
	}
	return int(numVal)
}
