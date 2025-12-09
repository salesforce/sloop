/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package server_metrics

import (
	"net/http"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// testOnce ensures metrics are only initialized once across all tests
var testOnce sync.Once

// initTestMetrics initializes metrics once for all tests using the default config
func initTestMetrics() {
	testOnce.Do(func() {
		// Reset in case previous test run left state
		if userRequestCounter != nil {
			prometheus.Unregister(userRequestCounter)
		}
		initOnce = sync.Once{}
		configuredLabels = nil
		configuredHeaderRules = nil
		userRequestCounter = nil

		InitUserMetrics(GetDefaultUserMetricsConfig())
	})
}

func TestGetHeaderWithFallback(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		headerNames []string
		want        string
	}{
		{
			name:        "no headers",
			headers:     map[string]string{},
			headerNames: []string{"x-email", "X-Email"},
			want:        "",
		},
		{
			name:        "first header present",
			headers:     map[string]string{"x-email": "test@example.com"},
			headerNames: []string{"x-email", "X-Email"},
			want:        "test@example.com",
		},
		{
			name:        "second header present",
			headers:     map[string]string{"X-Email": "uppercase@example.com"},
			headerNames: []string{"x-email", "X-Email"},
			want:        "uppercase@example.com",
		},
		{
			name:        "empty header value",
			headers:     map[string]string{"x-email": ""},
			headerNames: []string{"x-email", "X-Email"},
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := &http.Header{}

			for k, v := range tt.headers {
				headers.Set(k, v)
			}

			got := getHeaderWithFallback(headers, tt.headerNames)
			if got != tt.want {
				t.Errorf("getHeaderWithFallback() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDefaultUserMetricsConfig(t *testing.T) {
	config := GetDefaultUserMetricsConfig()

	if len(config) != 2 {
		t.Errorf("GetDefaultUserMetricsConfig() returned %d configs, want 2", len(config))
	}

	// Check username config
	usernameFound := false
	for _, c := range config {
		if c.Label == "username" {
			usernameFound = true
			if len(c.Headers) < 1 {
				t.Errorf("username config should have at least one header")
			}
		}
	}
	if !usernameFound {
		t.Errorf("GetDefaultUserMetricsConfig() should include username label")
	}

	// Check user_agent config
	userAgentFound := false
	for _, c := range config {
		if c.Label == "user_agent" {
			userAgentFound = true
		}
	}
	if !userAgentFound {
		t.Errorf("GetDefaultUserMetricsConfig() should include user_agent label")
	}
}

func TestInitUserMetricsIncludesBuiltInLabels(t *testing.T) {
	initTestMetrics()

	// Check that built-in labels are included
	labelMap := make(map[string]bool)
	for _, label := range configuredLabels {
		labelMap[label] = true
	}

	if !labelMap[LabelHandler] {
		t.Errorf("configuredLabels should include '%s'", LabelHandler)
	}
	if !labelMap[LabelCluster] {
		t.Errorf("configuredLabels should include '%s'", LabelCluster)
	}
	if !labelMap[LabelQuery] {
		t.Errorf("configuredLabels should include '%s'", LabelQuery)
	}
	if !labelMap[LabelNamespace] {
		t.Errorf("configuredLabels should include '%s'", LabelNamespace)
	}
	if !labelMap[LabelKind] {
		t.Errorf("configuredLabels should include '%s'", LabelKind)
	}
	if !labelMap[LabelName] {
		t.Errorf("configuredLabels should include '%s'", LabelName)
	}
	if !labelMap[LabelLookback] {
		t.Errorf("configuredLabels should include '%s'", LabelLookback)
	}

	// Check that header-based labels are also included
	if !labelMap["username"] {
		t.Errorf("configuredLabels should include 'username'")
	}
	if !labelMap["user_agent"] {
		t.Errorf("configuredLabels should include 'user_agent'")
	}
}

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name       string
		base       map[string]string
		additional map[string]string
		want       map[string]string
	}{
		{
			name:       "merge with override",
			base:       map[string]string{"key1": "value1", "key2": "value2"},
			additional: map[string]string{"key2": "newvalue2", "key3": "value3"},
			want:       map[string]string{"key1": "value1", "key2": "newvalue2", "key3": "value3"},
		},
		{
			name:       "empty additional",
			base:       map[string]string{"key1": "value1"},
			additional: map[string]string{},
			want:       map[string]string{"key1": "value1"},
		},
		{
			name:       "empty base",
			base:       map[string]string{},
			additional: map[string]string{"key1": "value1"},
			want:       map[string]string{"key1": "value1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeMaps(tt.base, tt.additional)
			if len(got) != len(tt.want) {
				t.Errorf("mergeMaps() map length = %d, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("mergeMaps() [%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestUserMetricsConfigJSON(t *testing.T) {
	// Test that the config structure can be represented correctly
	config := UserMetricsConfig{
		Label:   "email",
		Headers: []string{"X-Email", "X-User-Email", "x-email"},
	}

	if config.Label != "email" {
		t.Errorf("Expected label 'email', got '%s'", config.Label)
	}
	if len(config.Headers) != 3 {
		t.Errorf("Expected 3 headers, got %d", len(config.Headers))
	}
}

func TestExtractRequesterInfo(t *testing.T) {
	initTestMetrics()

	// Test with no headers
	headers := &http.Header{}
	info := extractRequesterInfo(headers)
	if info["username"] != "" {
		t.Errorf("Expected username to be empty, got %s", info["username"])
	}
	if info["user_agent"] != "" {
		t.Errorf("Expected user_agent to be empty, got %s", info["user_agent"])
	}

	// Test with headers
	headers.Set("X-Username", "testuser")
	headers.Set("User-Agent", "Mozilla/5.0")

	info = extractRequesterInfo(headers)
	if info["username"] != "testuser" {
		t.Errorf("Expected username to be 'testuser', got %s", info["username"])
	}
	if info["user_agent"] != "Mozilla/5.0" {
		t.Errorf("Expected user_agent to be 'Mozilla/5.0', got %s", info["user_agent"])
	}
}

func TestExtractRequesterInfoEmptyHeaders(t *testing.T) {
	initTestMetrics()

	headers := &http.Header{}
	headers.Set("X-Username", "")
	headers.Set("User-Agent", "")

	info := extractRequesterInfo(headers)
	if info["username"] != "" {
		t.Errorf("Expected username to be empty for empty header, got %s", info["username"])
	}
	if info["user_agent"] != "" {
		t.Errorf("Expected user_agent to be empty for empty header, got %s", info["user_agent"])
	}
}

func TestPublishHeaderMetrics(t *testing.T) {
	initTestMetrics()

	// Test with metrics disabled - should not panic
	headers := &http.Header{}
	headers.Set("X-Username", "testuser")
	additionalTags := map[string]string{
		LabelHandler:   "test",
		LabelCluster:   "test-cluster",
		LabelQuery:     "EventHeatMap",
		LabelNamespace: "default",
		LabelKind:      "Pod",
		LabelName:      "",
		LabelLookback:  "1h",
	}

	// Should not panic when disabled
	PublishHeaderMetrics(headers, additionalTags, false)

	// Should not panic when enabled
	PublishHeaderMetrics(headers, additionalTags, true)
}

func TestLabelCount(t *testing.T) {
	initTestMetrics()

	// Default config has 2 header labels (username, user_agent) + 7 built-in labels = 9 total
	expectedCount := 2 + len(builtInLabels)
	if len(configuredLabels) != expectedCount {
		t.Errorf("Expected %d configured labels with default config, got %d: %v",
			expectedCount, len(configuredLabels), configuredLabels)
	}
}
