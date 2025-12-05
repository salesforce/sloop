/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/salesforce/sloop/pkg/sloop/server/server_metrics"
)

const requestIDKey string = "reqId"

var (
	metricWebServerRequestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sloop_http_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "handler"},
	)
	metricWebServerRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sloop_http_request_duration_seconds",
			Help:    "A histogram of latencies for requests to the wrapped handler.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"handler"},
	)
)

func getRequestId(webContext context.Context) string {
	requestID, ok := webContext.Value(requestIDKey).(string)
	if !ok {
		requestID = "unknown"
	}
	return requestID
}

// Sets a request id in the context which can be used for logging
func traceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano()/1000)
		}
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logs all HTTP requests to glog
func glogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		before := time.Now()
		next.ServeHTTP(w, r)
		requestID := getRequestId(r.Context())
		timeTaken := time.Since(before)
		glog.Infof("reqId: %v http url: %v took: %v remote: %v useragent: %v", requestID, r.URL, timeTaken, r.RemoteAddr, r.UserAgent())
	})
}

// userMetricsMiddleware collects user metrics from request headers
func userMetricsMiddleware(handlerName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Call the next handler first
		next.ServeHTTP(w, r)

		// Collect metrics after the request is processed
		additionalTags := map[string]string{
			"handler": handlerName,
		}
		server_metrics.PublishHeaderMetrics(&r.Header, additionalTags, enableUserMetrics)
	})
}

func middlewareChain(handlerName string, next http.Handler) http.HandlerFunc {
	return promhttp.InstrumentHandlerCounter(
		metricWebServerRequestCount.MustCurryWith(prometheus.Labels{"handler": handlerName}),
		promhttp.InstrumentHandlerDuration(
			metricWebServerRequestDuration.MustCurryWith(prometheus.Labels{"handler": handlerName}),
			traceMiddleware(
				glogMiddleware(
					userMetricsMiddleware(handlerName, next),
				),
			),
		),
	)
}

func metricCountMiddleware(handlerName string, next http.Handler) http.HandlerFunc {
	return promhttp.InstrumentHandlerCounter(
		metricWebServerRequestCount.MustCurryWith(prometheus.Labels{"handler": handlerName}),
		userMetricsMiddleware(handlerName, next))
}
