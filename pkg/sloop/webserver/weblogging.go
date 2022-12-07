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
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const requestIDKey string = "reqId"

func getRequestId(webContext context.Context) string {
	requestID, ok := webContext.Value(requestIDKey).(string)
	if !ok {
		requestID = "unknown"
	}
	return requestID
}

// Sets a request id in the context which can be used for logging
func traceWrapper(next http.Handler) http.Handler {
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
func glogWrapper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		before := time.Now()
		next.ServeHTTP(w, r)
		requestID := getRequestId(r.Context())
		timeTaken := time.Since(before)
		glog.Infof("reqId: %v http url: %v took: %v remote: %v useragent: %v", requestID, r.URL, timeTaken, r.RemoteAddr, r.UserAgent())
	})
}

func metricCountsDurationsWrapperChain(handler string, next http.Handler) http.HandlerFunc {
	return promhttp.InstrumentHandlerCounter(
		metricWebServerRequestCount.MustCurryWith(prometheus.Labels{"handler": handler}),
		promhttp.InstrumentHandlerDuration(
			metricWebServerRequestDuration.MustCurryWith(prometheus.Labels{"handler": handler}), next))
}

func metricCountsWrapper(handler string, next http.Handler) http.HandlerFunc {
	return promhttp.InstrumentHandlerCounter(
		metricWebServerRequestCount.MustCurryWith(prometheus.Labels{"handler": handler}), next)
}

func metricDurationsWrapper(handler string, next http.Handler) http.HandlerFunc {
	return promhttp.InstrumentHandlerDuration(
		metricWebServerRequestDuration.MustCurryWith(prometheus.Labels{"handler": handler}), next)
}
