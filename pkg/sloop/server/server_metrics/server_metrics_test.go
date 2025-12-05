/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package server_metrics

import (
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

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

func TestExtractRequesterInfo(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    map[string]string
	}{
		{
			name:    "username present",
			headers: map[string]string{HeaderXUsername: "testuser", HeaderUserAgent: "test-agent"},
			want: map[string]string{
				KeyUsername:  "testuser",
				KeyUserAgent: "test-agent",
			},
		},
		{
			name: "no username, has XFCC with service and instance",
			headers: map[string]string{
				HeaderXForwardedClientCert: "By=spiffe://example.com/service=myservice/service_instance=instance1;Hash=abc123;URI=spiffe://example.com/service=myservice/service_instance=instance1",
				HeaderUserAgent:            "test-agent",
			},
			want: map[string]string{
				KeyURIService:         "myservice",
				KeyURIServiceInstance: "instance1",
				KeyUserAgent:          "test-agent",
			},
		},
		{
			name: "no username, has XFCC with only service",
			headers: map[string]string{
				HeaderXForwardedClientCert: "By=spiffe://example.com/service=myservice;Hash=abc123;URI=spiffe://example.com/service=myservice",
				HeaderUserAgent:            "test-agent",
			},
			want: map[string]string{
				KeyURIService:         "myservice",
				KeyURIServiceInstance: "",
				KeyUserAgent:          "test-agent",
			},
		},
		{
			name: "no username, has XFCC without URI",
			headers: map[string]string{
				HeaderXForwardedClientCert: "By=spiffe://example.com;Hash=abc123",
				HeaderUserAgent:            "test-agent",
			},
			want: map[string]string{
				KeyURIService:         "",
				KeyURIServiceInstance: "",
				KeyUserAgent:          "test-agent",
			},
		},
		{
			name:    "no username, no XFCC",
			headers: map[string]string{HeaderUserAgent: "test-agent"},
			want: map[string]string{
				KeyURIService:         "",
				KeyURIServiceInstance: "",
				KeyUserAgent:          "test-agent",
			},
		},
		{
			name:    "case insensitive username",
			headers: map[string]string{HeaderXUsernameLower: "testuser", HeaderUserAgent: "test-agent"},
			want: map[string]string{
				KeyUsername:  "testuser",
				KeyUserAgent: "test-agent",
			},
		},
		{
			name: "XFCC with spaces in service names",
			headers: map[string]string{
				HeaderXForwardedClientCert: "By=spiffe://example.com/service= my service /service_instance= my instance ;Hash=abc123;URI=spiffe://example.com/service= my service /service_instance= my instance ",
				HeaderUserAgent:            "test-agent",
			},
			want: map[string]string{
				KeyURIService:         "my service",
				KeyURIServiceInstance: "my instance",
				KeyUserAgent:          "test-agent",
			},
		},
		{
			name: "XFCC with special characters",
			headers: map[string]string{
				HeaderXForwardedClientCert: "By=spiffe://example.com/service=my-service_v1/service_instance=instance-1.0;Hash=abc123;URI=spiffe://example.com/service=my-service_v1/service_instance=instance-1.0",
				HeaderUserAgent:            "test-agent",
			},
			want: map[string]string{
				KeyURIService:         "my-service_v1",
				KeyURIServiceInstance: "instance-1.0",
				KeyUserAgent:          "test-agent",
			},
		},
		{
			name: "XFCC with comma-separated values (regex stops at = or /)",
			headers: map[string]string{
				HeaderXForwardedClientCert: "By=spiffe://cluster.local/ns/default/sa/default;Hash=abc123;Subject=\"\";URI=spiffe://cluster.local/ns/default/sa/default/service=my-service,service_instance=my-instance",
				HeaderUserAgent:            "test-agent",
			},
			want: map[string]string{
				KeyURIService:         "my-service,service_instance",
				KeyURIServiceInstance: "my-instance", // extractServiceFromURI extracts this separately
				KeyUserAgent:          "test-agent",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := &http.Header{}

			for k, v := range tt.headers {
				headers.Set(k, v)
			}

			got := extractRequesterInfo(headers)

			// Compare maps
			if len(got) != len(tt.want) {
				t.Errorf("extractRequesterInfo() map length = %d, want %d. Got: %v, Want: %v", len(got), len(tt.want), got, tt.want)
			}

			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("extractRequesterInfo() [%s] = %v, want %v. Full map: %v", k, got[k], v, got)
				}
			}

			// Check that username and service are mutually exclusive
			if got[KeyUsername] != "" && got[KeyURIService] != "" {
				t.Errorf("extractRequesterInfo() should not have both username and uri_service set")
			}
		})
	}
}

func TestExtractXFCCInfo(t *testing.T) {
	tests := []struct {
		name            string
		xfccHeader      string
		wantService     string
		wantServiceInst string
	}{
		{
			name:            "no XFCC header",
			xfccHeader:      "",
			wantService:     "",
			wantServiceInst: "",
		},
		{
			name:            "XFCC with service and instance",
			xfccHeader:      "By=spiffe://cluster.local/ns/default/sa/default;Hash=abc123;Subject=\"\";URI=spiffe://cluster.local/ns/default/sa/default/service=my-service/service_instance=my-instance",
			wantService:     "my-service",
			wantServiceInst: "my-instance",
		},
		{
			name:            "XFCC with only service",
			xfccHeader:      "By=spiffe://cluster.local/ns/default/sa/default;Hash=abc123;Subject=\"\";URI=spiffe://cluster.local/ns/default/sa/default/service=my-service",
			wantService:     "my-service",
			wantServiceInst: "",
		},
		{
			name:            "XFCC without URI",
			xfccHeader:      "By=spiffe://cluster.local/ns/default/sa/default;Hash=abc123",
			wantService:     "",
			wantServiceInst: "",
		},
		{
			name:            "malformed XFCC header",
			xfccHeader:      "malformed-header-without-proper-format",
			wantService:     "",
			wantServiceInst: "",
		},
		{
			name:            "XFCC with comma-separated values in URI",
			xfccHeader:      "By=spiffe://cluster.local/ns/default/sa/default;Hash=abc123;Subject=\"\";URI=spiffe://cluster.local/ns/default/sa/default/service=my-service,service_instance=my-instance",
			wantService:     "my-service,service_instance",
			wantServiceInst: "my-instance", // extractServiceFromURI extracts this separately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := &http.Header{}
			if tt.xfccHeader != "" {
				headers.Set(HeaderXForwardedClientCert, tt.xfccHeader)
			}

			gotService, gotServiceInst := extractXFCCInfo(headers)
			if gotService != tt.wantService {
				t.Errorf("extractXFCCInfo() service = %v, want %v", gotService, tt.wantService)
			}
			if gotServiceInst != tt.wantServiceInst {
				t.Errorf("extractXFCCInfo() serviceInstance = %v, want %v", gotServiceInst, tt.wantServiceInst)
			}
		})
	}
}

func TestExtractField(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		field string
		want  string
	}{
		{
			name:  "extract service from comma-separated",
			text:  "service=my-service,service_instance=my-instance,other=value",
			field: "service=",
			want:  "my-service,service_instance", // Stops at next =, not comma
		},
		{
			name:  "extract service_instance from comma-separated",
			text:  "service=my-service,service_instance=my-instance,other=value",
			field: "service_instance=",
			want:  "my-instance,other", // Stops at next =, not comma
		},
		{
			name:  "extract service with slash separator",
			text:  "service=my-service/service_instance=my-instance",
			field: "service=",
			want:  "my-service", // Stops at /
		},
		{
			name:  "extract service_instance with slash separator",
			text:  "service=my-service/service_instance=my-instance",
			field: "service_instance=",
			want:  "my-instance",
		},
		{
			name:  "non-existent field",
			text:  "service=my-service,service_instance=my-instance",
			field: "non_existent=",
			want:  "",
		},
		{
			name:  "field with spaces",
			text:  "service= my service /service_instance= my instance ",
			field: "service=",
			want:  "my service", // Stops at /
		},
		{
			name:  "field with special characters",
			text:  "service=my-service_v1/service_instance=instance-1.0",
			field: "service=",
			want:  "my-service_v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractField(tt.text, tt.field)
			if got != tt.want {
				t.Errorf("extractField(%q, %q) = %v, want %v", tt.text, tt.field, got, tt.want)
			}
		})
	}
}

func TestExtractServiceFromURI(t *testing.T) {
	tests := []struct {
		name            string
		uri             string
		wantService     string
		wantServiceInst string
	}{
		{
			name:            "both service and service_instance with slash",
			uri:             "spiffe://cluster.local/ns/default/sa/default/service=my-service/service_instance=my-instance",
			wantService:     "my-service",
			wantServiceInst: "my-instance",
		},
		{
			name:            "only service with slash",
			uri:             "spiffe://cluster.local/ns/default/sa/default/service=my-service",
			wantService:     "my-service",
			wantServiceInst: "",
		},
		{
			name:            "no service",
			uri:             "spiffe://cluster.local/ns/default/sa/default",
			wantService:     "",
			wantServiceInst: "",
		},
		{
			name:            "both service and service_instance with comma",
			uri:             "spiffe://cluster.local/ns/default/sa/default/service=my-service,service_instance=my-instance",
			wantService:     "my-service,service_instance",
			wantServiceInst: "my-instance", // extractField extracts this separately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotService, gotServiceInst := extractServiceFromURI(tt.uri)
			if gotService != tt.wantService {
				t.Errorf("extractServiceFromURI() service = %v, want %v", gotService, tt.wantService)
			}
			if gotServiceInst != tt.wantServiceInst {
				t.Errorf("extractServiceFromURI() serviceInstance = %v, want %v", gotServiceInst, tt.wantServiceInst)
			}
		})
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

func TestPublishHeaderMetrics(t *testing.T) {
	tests := []struct {
		name           string
		enableMetrics  bool
		headers        map[string]string
		additionalTags map[string]string
		expectedMetric string
		expectedCount  int
		expectedLabels []string
	}{
		{
			name:          "username present with metrics enabled",
			enableMetrics: true,
			headers: map[string]string{
				HeaderXUsername: "testuser",
				HeaderUserAgent: "test-agent",
			},
			additionalTags: map[string]string{},
			expectedMetric: MetricUserRequests,
			expectedCount:  1,
			expectedLabels: []string{"testuser", "test-agent", "", ""},
		},
		{
			name:          "no username, no service with metrics enabled",
			enableMetrics: true,
			headers: map[string]string{
				HeaderUserAgent: "test-agent",
			},
			additionalTags: map[string]string{},
			expectedMetric: "",
			expectedCount:  0,
			expectedLabels: nil,
		},
		{
			name:          "metrics disabled",
			enableMetrics: false,
			headers: map[string]string{
				HeaderXUsername: "testuser",
				HeaderUserAgent: "test-agent",
			},
			additionalTags: map[string]string{},
			expectedMetric: "",
			expectedCount:  0,
			expectedLabels: nil,
		},
		{
			name:          "XFCC header with metrics enabled",
			enableMetrics: true,
			headers: map[string]string{
				HeaderXForwardedClientCert: "By=spiffe://example.com/service=myservice/service_instance=instance1;Hash=abc123;URI=spiffe://example.com/service=myservice/service_instance=instance1",
				HeaderUserAgent:            "test-agent",
			},
			additionalTags: map[string]string{},
			expectedMetric: MetricServiceRequests,
			expectedCount:  1,
			expectedLabels: []string{"test-agent", "myservice", "instance1"},
		},
		{
			name:          "XFCC with only service",
			enableMetrics: true,
			headers: map[string]string{
				HeaderXForwardedClientCert: "By=spiffe://example.com/service=myservice;Hash=abc123;URI=spiffe://example.com/service=myservice",
				HeaderUserAgent:            "test-agent",
			},
			additionalTags: map[string]string{},
			expectedMetric: MetricServiceRequests,
			expectedCount:  1,
			expectedLabels: []string{"test-agent", "myservice", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset metrics before test
			userRequestCounter.Reset()
			serviceRequestCounter.Reset()

			// Set up headers
			headers := &http.Header{}
			for k, v := range tt.headers {
				headers.Set(k, v)
			}

			// Call the function
			PublishHeaderMetrics(headers, tt.additionalTags, tt.enableMetrics)

			// Check results
			if tt.expectedMetric == MetricUserRequests {
				count := testutil.ToFloat64(userRequestCounter.WithLabelValues(tt.expectedLabels...))
				if int(count) != tt.expectedCount {
					t.Errorf("PublishHeaderMetrics() userRequestCounter count = %d, want %d", int(count), tt.expectedCount)
				}
			} else if tt.expectedMetric == MetricServiceRequests {
				count := testutil.ToFloat64(serviceRequestCounter.WithLabelValues(tt.expectedLabels...))
				if int(count) != tt.expectedCount {
					t.Errorf("PublishHeaderMetrics() serviceRequestCounter count = %d, want %d", int(count), tt.expectedCount)
				}
			} else {
				// No metric should be published
				userCount := testutil.CollectAndCount(userRequestCounter)
				serviceCount := testutil.CollectAndCount(serviceRequestCounter)
				if userCount+serviceCount != 0 {
					t.Errorf("PublishHeaderMetrics() should not publish metrics, but got userCount=%d, serviceCount=%d", userCount, serviceCount)
				}
			}
		})
	}
}

// Legacy compatibility tests
func TestExtractUserInfo(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/v1/pods", nil)

	// Test with no headers
	userInfo := extractUserInfo(req)
	if userInfo.Email != "" {
		t.Errorf("Expected email to be empty, got %s", userInfo.Email)
	}
	if userInfo.Username != "" {
		t.Errorf("Expected username to be empty, got %s", userInfo.Username)
	}
	if userInfo.UserAgent != "" {
		t.Errorf("Expected user agent to be empty, got %s", userInfo.UserAgent)
	}

	// Test with headers
	req.Header.Set("x-username", "testuser")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	userInfo = extractUserInfo(req)
	if userInfo.Username != "testuser" {
		t.Errorf("Expected username to be 'testuser', got %s", userInfo.Username)
	}
	if userInfo.UserAgent != "Mozilla/5.0" {
		t.Errorf("Expected user agent to be 'Mozilla/5.0', got %s", userInfo.UserAgent)
	}
}

func TestExtractUserInfoCaseSensitivity(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/v1/pods", nil)

	// Test case sensitivity - HTTP headers are case-insensitive, so setting both
	// will result in the last one set being returned
	req.Header.Set("x-username", "lowercase")
	req.Header.Set("X-Username", "uppercase")

	userInfo := extractUserInfo(req)
	// HTTP headers are case-insensitive, so the last one set wins
	if userInfo.Username != "uppercase" {
		t.Errorf("Expected username to be 'uppercase', got %s", userInfo.Username)
	}
}

func TestExtractUserInfoEmptyHeaders(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/v1/pods", nil)

	// Test with empty header values
	req.Header.Set("x-username", "")
	req.Header.Set("User-Agent", "")

	userInfo := extractUserInfo(req)
	if userInfo.Username != "" {
		t.Errorf("Expected username to be empty for empty header, got %s", userInfo.Username)
	}
	if userInfo.UserAgent != "" {
		t.Errorf("Expected user agent to be empty for empty header, got %s", userInfo.UserAgent)
	}
}
