/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package server_metrics

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Header names
	HeaderXUsername            = "X-Username"
	HeaderXUsernameLower       = "x-username"
	HeaderUserAgent            = "User-Agent"
	HeaderXForwardedClientCert = "X-Forwarded-Client-Cert"

	// Metric names
	MetricUserRequests    = "user_requests"
	MetricServiceRequests = "service_requests"

	// label keys
	KeyUsername           = "username"
	KeyUserAgent          = "user_agent"
	KeyURIService         = "uri_service"
	KeyURIServiceInstance = "uri_service_instance"

	// URI field names
	FieldService         = "service="
	FieldServiceInstance = "service_instance="
)

var (
	userRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricUserRequests,
			Help: "Counter of dashboard requests broken out by user information from headers.",
		},
		[]string{KeyUsername, KeyUserAgent, KeyURIService, KeyURIServiceInstance},
	)

	serviceRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricServiceRequests,
			Help: "Counter of dashboard requests broken out by service information from headers.",
		},
		[]string{KeyUserAgent, KeyURIService, KeyURIServiceInstance},
	)
)

// Initialize user metrics in prometheus
func init() {
	prometheus.MustRegister(userRequestCounter)
	prometheus.MustRegister(serviceRequestCounter)
}

// EnsureUserMetricsRegistered ensures user metrics are registered
// This can be called after configuration is loaded to ensure proper initialization
func EnsureUserMetricsRegistered() {
	// Metrics are already registered in init(), but this provides a hook
	// for any future initialization needs
}

// ============================================================================
// Header Information Extraction Functions
// ============================================================================

// These functions handle extracting user information from HTTP request headers

// getHeaderWithFallback tries multiple header names and returns the first non-empty value or empty string if no value is found
func getHeaderWithFallback(headers *http.Header, headerNames []string) string {
	for _, headerName := range headerNames {
		if value := headers.Get(headerName); value != "" {
			return value
		}
	}
	return ""
}

// extractRequesterInfo extracts user/service information from request headers and returns a map with requester information
func extractRequesterInfo(headers *http.Header) map[string]string {
	requesterInfo := map[string]string{}

	// Extract username from headers if present otherwise extract service information from URI
	userName := getHeaderWithFallback(headers, []string{HeaderXUsernameLower, HeaderXUsername})
	if userName != "" {
		requesterInfo[KeyUsername] = userName
	} else {
		// Extract XFCC information
		requesterInfo[KeyURIService], requesterInfo[KeyURIServiceInstance] = extractXFCCInfo(headers)
	}

	requesterInfo[KeyUserAgent] = getHeaderWithFallback(headers, []string{HeaderUserAgent})

	return requesterInfo
}

// extractXFCCInfo extracts service information from X-Forwarded-Client-Cert header
func extractXFCCInfo(headers *http.Header) (service, serviceInstance string) {
	// Initialize with default values
	service = ""
	serviceInstance = ""

	clientCert := headers.Get(HeaderXForwardedClientCert)
	if clientCert == "" {
		glog.V(4).Infof("X-Forwarded-Client-Cert header not found")
		return service, serviceInstance
	}

	glog.V(4).Infof("X-Forwarded-Client-Cert header found: %s", clientCert)

	// Parse XFCC header - look for URI field with service information
	// The XFCC header format is: By=...;Hash=...;Subject=...;URI=spiffe://.../service=value,service_instance=value
	if strings.Contains(clientCert, "URI=") {
		// Extract the URI portion
		uriStart := strings.Index(clientCert, "URI=")
		if uriStart != -1 {
			uriPart := clientCert[uriStart+4:] // Skip "URI="

			// Find the end of the URI (next semicolon or end of string)
			if semicolonIdx := strings.Index(uriPart, ";"); semicolonIdx != -1 {
				uriPart = uriPart[:semicolonIdx]
			}

			service, serviceInstance = extractServiceFromURI(uriPart)
			if service != "" {
				glog.V(4).Infof("Parsed XFCC info - service: '%s', service_instance: '%s'", service, serviceInstance)
				return service, serviceInstance
			}
		}
	}

	// If parsing didn't work, return empty values
	glog.V(4).Infof("No service information found in XFCC header")
	return service, serviceInstance
}

// extractServiceFromURI extracts service and service instance from SPIFFE URI
func extractServiceFromURI(uri string) (service, serviceInstance string) {
	// Extract service=value from URI
	service = extractField(uri, FieldService)
	if service != "" {
		// Extract service_instance=value from URI
		serviceInstance = extractField(uri, FieldServiceInstance)
		return service, serviceInstance
	}

	// If no service found, return empty strings
	return "", ""
}

// extractField extracts a field value from a string using regex
func extractField(text, field string) string {
	// Create regex pattern to match field=value, stopping at next / or = or end of string
	pattern := regexp.MustCompile(field + `([^/=]+)`)
	matches := pattern.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// mergeMaps merges two maps, with values from additionalTags taking precedence
func mergeMaps(base, additional map[string]string) map[string]string {
	result := make(map[string]string)

	// Copy base map
	for k, v := range base {
		result[k] = v
	}

	// Override with additional tags
	for k, v := range additional {
		result[k] = v
	}

	return result
}

// publishHeaderMetrics publishes header metrics to the metrics server
func PublishHeaderMetrics(headers *http.Header, additionalTags map[string]string, enableUserMetrics bool) {
	if !enableUserMetrics {
		return
	}

	userInfo := extractRequesterInfo(headers)

	// merge the userInfo and additionalTags maps into a new map
	labels := mergeMaps(userInfo, additionalTags)

	// if username is present it is a user request
	if labels[KeyUsername] != "" {
		userRequestCounter.WithLabelValues(
			labels[KeyUsername],
			labels[KeyUserAgent],
			labels[KeyURIService],
			labels[KeyURIServiceInstance],
		).Inc()
		return
	}

	// if uri_service is present it is a service request
	if labels[KeyURIService] != "" {
		serviceRequestCounter.WithLabelValues(
			labels[KeyUserAgent],
			labels[KeyURIService],
			labels[KeyURIServiceInstance],
		).Inc()
		return
	}

	glog.Warningf("Failed to determine the request type")
}

// ============================================================================
// Legacy compatibility functions
// ============================================================================

// These functions maintain compatibility with the existing filter.go code

// UserInfo contains user information extracted from request headers
type UserInfo struct {
	Email           string
	Username        string
	UserAgent       string
	Service         string
	ServiceInstance string
}

// requestMeta contains metadata about the request for metrics
type requestMeta struct {
	userInfo UserInfo
}

// extractUserInfo extracts user information from request headers and returns a UserInfo struct
// This is a compatibility wrapper for the existing filter.go code
func extractUserInfo(request *http.Request) UserInfo {
	userInfo := UserInfo{}

	// Extract standard headers
	userInfo.Username = getHeaderWithFallback(&request.Header, []string{HeaderXUsernameLower, HeaderXUsername})
	userInfo.UserAgent = getHeaderWithFallback(&request.Header, []string{HeaderUserAgent})

	// Extract XFCC information
	userInfo.Service, userInfo.ServiceInstance = extractXFCCInfo(&request.Header)

	return userInfo
}

// publishUserMetrics publishes user-specific metrics if enabled
// This is a compatibility wrapper for the existing filter.go code
func publishUserMetrics(requestMeta requestMeta, resourceRequest *string, enableUserMetrics bool) {
	if !enableUserMetrics {
		return
	}

	// Build additional tags from resource request
	additionalTags := make(map[string]string)
	if resourceRequest != nil {
		additionalTags["resource"] = *resourceRequest
	}

	// Create a temporary request to extract headers
	// Since we already have the userInfo, we'll construct the labels directly
	labels := make(map[string]string)
	labels[KeyUserAgent] = requestMeta.userInfo.UserAgent
	labels[KeyURIService] = requestMeta.userInfo.Service
	labels[KeyURIServiceInstance] = requestMeta.userInfo.ServiceInstance

	// if username is present it is a user request
	if requestMeta.userInfo.Username != "" {
		labels[KeyUsername] = requestMeta.userInfo.Username
		userRequestCounter.WithLabelValues(
			labels[KeyUsername],
			labels[KeyUserAgent],
			labels[KeyURIService],
			labels[KeyURIServiceInstance],
		).Inc()
		return
	}

	// if uri_service is present it is a service request
	if labels[KeyURIService] != "" {
		serviceRequestCounter.WithLabelValues(
			labels[KeyUserAgent],
			labels[KeyURIService],
			labels[KeyURIServiceInstance],
		).Inc()
		return
	}

	glog.Warningf("Failed to determine the request type")
}
