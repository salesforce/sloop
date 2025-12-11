/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package server_metrics

import (
	"net/http"
	"sort"
	"sync"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Metric names
	MetricUserRequests = "user_requests"

	// Built-in request context label keys (always included)
	LabelHandler   = "handler"
	LabelCluster   = "cluster"
	LabelQuery     = "query"
	LabelNamespace = "namespace"
	LabelKind      = "kind"
	LabelName      = "name"
	LabelLookback  = "lookback"
)

// UserMetricsConfig defines a single header extraction rule for user metrics
type UserMetricsConfig struct {
	// Label is the Prometheus metric label name (e.g., "username", "email", "team")
	Label string `json:"label"`
	// Headers is a list of HTTP header names to check, in order of preference
	// The first non-empty header value found will be used
	Headers []string `json:"headers"`
}

var (
	userRequestCounter    *prometheus.CounterVec
	configuredLabels      []string
	configuredHeaderRules []UserMetricsConfig
	initOnce              sync.Once

	// Built-in labels that are always included for request context
	builtInLabels = []string{LabelHandler, LabelCluster, LabelQuery, LabelNamespace, LabelKind, LabelName, LabelLookback}
)

// GetDefaultUserMetricsConfig returns the default configuration for user metrics (username and user_agent)
func GetDefaultUserMetricsConfig() []UserMetricsConfig {
	return []UserMetricsConfig{
		{
			Label:   "username",
			Headers: []string{"X-Username", "x-username"},
		},
		{
			Label:   "user_agent",
			Headers: []string{"User-Agent"},
		},
	}
}

// InitUserMetrics initializes the user metrics with the given configuration
// This should be called once during application startup before serving requests
// If config is nil or empty, default configuration will be used
func InitUserMetrics(config []UserMetricsConfig) {
	initOnce.Do(func() {
		// Use default config if none provided
		if len(config) == 0 {
			config = GetDefaultUserMetricsConfig()
			glog.Infof("No user metrics configuration provided, using defaults")
		}

		// Store the configuration
		configuredHeaderRules = config

		// Start with built-in labels for request context
		labelSet := make(map[string]bool)
		for _, label := range builtInLabels {
			labelSet[label] = true
		}

		// Add header-based labels from configuration
		for _, rule := range config {
			if rule.Label != "" {
				labelSet[rule.Label] = true
			}
		}

		// Sort labels for consistent ordering
		configuredLabels = make([]string, 0, len(labelSet))
		for label := range labelSet {
			configuredLabels = append(configuredLabels, label)
		}
		sort.Strings(configuredLabels)

		// Create the counter with configured labels
		userRequestCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricUserRequests,
				Help: "Counter of requests broken out by user and request information.",
			},
			configuredLabels,
		)

		// Register with Prometheus
		prometheus.MustRegister(userRequestCounter)

		glog.Infof("User metrics initialized with labels: %v", configuredLabels)
		glog.Infof("  Built-in request labels: %v", builtInLabels)
		for _, rule := range config {
			glog.Infof("  Header label '%s': headers=%v", rule.Label, rule.Headers)
		}
	})
}

// ============================================================================
// Header Information Extraction Functions
// ============================================================================

// getHeaderWithFallback tries multiple header names and returns the first non-empty value
func getHeaderWithFallback(headers *http.Header, headerNames []string) string {
	for _, headerName := range headerNames {
		if value := headers.Get(headerName); value != "" {
			return value
		}
	}
	return ""
}

// extractRequesterInfo extracts user information from request headers based on configuration
func extractRequesterInfo(headers *http.Header) map[string]string {
	requesterInfo := make(map[string]string)

	for _, rule := range configuredHeaderRules {
		if rule.Label != "" && len(rule.Headers) > 0 {
			requesterInfo[rule.Label] = getHeaderWithFallback(headers, rule.Headers)
		}
	}

	return requesterInfo
}

// mergeMaps merges two maps, with values from additionalTags taking precedence
func mergeMaps(base, additional map[string]string) map[string]string {
	result := make(map[string]string)

	for k, v := range base {
		result[k] = v
	}

	for k, v := range additional {
		result[k] = v
	}

	return result
}

// PublishHeaderMetrics publishes header metrics to the metrics server
func PublishHeaderMetrics(headers *http.Header, additionalTags map[string]string, enableUserMetrics bool) {
	if !enableUserMetrics {
		return
	}

	userInfo := extractRequesterInfo(headers)
	allLabels := mergeMaps(userInfo, additionalTags)

	// Build label values in the correct order
	labelValues := make([]string, len(configuredLabels))
	for i, label := range configuredLabels {
		labelValues[i] = allLabels[label]
	}

	userRequestCounter.WithLabelValues(labelValues...).Inc()

	// Log user request at V(2) verbosity level for debugging
	glog.V(2).Infof("User request made with labels: %v", allLabels)
}
