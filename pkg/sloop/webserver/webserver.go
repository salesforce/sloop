/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"context"
	"expvar"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/salesforce/sloop/pkg/sloop/queries"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"golang.org/x/net/trace"
)

const (
	debugViewKeyTemplateFile      = "debugviewkey.html"
	debugListKeysTemplateFile     = "debuglistkeys.html"
	debugHistogramFile            = "debughistogram.html"
	debugConfigTemplateFile       = "debugconfig.html"
	debugTemplateFile             = "debug.html"
	debugBadgerTablesTemplateFile = "debugtables.html"
	indexTemplateFile             = "index.html"
	resourceTemplateFile          = "resource.html"
)

type WebConfig struct {
	Port             int
	WebFilesPath     string
	DefaultNamespace string
	DefaultLookback  string
	DefaultResources string
	MaxLookback      time.Duration
	ConfigYaml       string
	ResourceLinks    []ResourceLinkTemplate
	LeftBarLinks     []LinkTemplate
	CurrentContext   string
}

var (
	metricWebServerRequestCount   = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_webserver_request_count"})
	metricWebServerRequestLatency = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_webserver_request_latency_sec"})
)

// This is not going to change and we don't want to pass it to every function
// so use a static for now
var webFiles string

// Needed to use this to allow for graceful shutdown which is required for profiling
type Server struct {
	mux *http.ServeMux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func logWebError(err error, note string, r *http.Request, w http.ResponseWriter) {
	message := fmt.Sprintf("Error rendering url: %q.  Note: %v. Error: %v", r.URL, note, err)
	glog.ErrorDepth(1, message)
	http.Error(w, message, http.StatusInternalServerError)
}

// TODO: We should probably only allow a fixed set of files to read so users don't get creative

// Example input: r.URL=/webfiles/static/style.css
// Returns file: <webFiles>/static/style.css
func webFileHandler(w http.ResponseWriter, r *http.Request) {
	fixedUrl := strings.TrimPrefix(fmt.Sprint(r.URL), "/webfiles")
	if strings.Contains(fixedUrl, "..") {
		logWebError(nil, "Not allowed", r, w)
		return
	}
	fullPath := path.Join(webFiles, fixedUrl)
	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		logWebError(err, "Error reading web file: "+fixedUrl, r, w)
		return
	}
	w.Header().Set("content-type", mime.TypeByExtension(filepath.Ext(fullPath)))
	_, err = w.Write(data)
	if err != nil {
		logWebError(err, "Error writing web file: "+fixedUrl, r, w)
		return
	}
	glog.V(2).Infof("webFileHandler successfully returned file %v for %v", fixedUrl, r.URL)
}

// backupHandler streams a download of a backup of the database.
// It is a simple HTTP translation of the Badger DB's built-in online backup function.
// If the optional `since` query parameter is provided, the backup will only include versions since the version provided.
func backupHandler(db badgerwrap.DB, currentContext string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sinceStr := r.URL.Query().Get("since")
		if sinceStr == "" {
			sinceStr = "0"
		}
		since, err := strconv.ParseUint(sinceStr, 10, 64)
		if err != nil {
			logWebError(err, "Error parsing 'since' parameter. Must be expressed as a positive integer.", r, w)
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=sloop-%s-%d.bak", currentContext, since))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Transfer-Encoding", "chunked")

		_, err = db.Backup(w, since)
		if err != nil {
			logWebError(err, "Error writing backup", r, w)
			return
		}

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

// Returns json to feed into dhtmlgantt
// Info on data format: https://docs.dhtmlx.com/gantt/desktop__loading.html

func queryHandler(tables typed.Tables, maxLookBack time.Duration) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("content-type", "application/json")

		queryName := request.URL.Query().Get(queries.QueryParam)
		data, err := queries.RunQuery(queryName, request.URL.Query(), tables, maxLookBack, getRequestId(request.Context()))
		if err != nil {
			logWebError(err, "Failed to run query", request, writer)
			return
		}

		writer.Write(data)
	}
}

func healthHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		writer.Write([]byte(http.StatusText(http.StatusOK)))
	}
}

func Run(config WebConfig, tables typed.Tables) error {
	webFiles = config.WebFilesPath
	server := &Server{}
	server.mux = http.NewServeMux()
	server.mux.HandleFunc("/webfiles/", webFileHandler)
	server.mux.HandleFunc("/data/backup", backupHandler(tables.Db(), config.CurrentContext))
	server.mux.HandleFunc("/data", queryHandler(tables, config.MaxLookback))
	server.mux.HandleFunc("/resource", resourceHandler(config.ResourceLinks))
	// Debug pages
	server.mux.HandleFunc("/debug/", debugHandler())
	server.mux.HandleFunc("/debug/listkeys/", listKeysHandler(tables))
	server.mux.HandleFunc("/debug/histogram/", histogramHandler(tables))
	server.mux.HandleFunc("/debug/tables/", debugBadgerTablesHandler(tables.Db()))
	server.mux.HandleFunc("/debug/view/", viewKeyHandler(tables))
	server.mux.HandleFunc("/debug/config/", configHandler(config.ConfigYaml))
	// Badger uses the trace package, which registers /debug/requests and /debug/events
	server.mux.HandleFunc("/debug/requests", trace.Traces)
	server.mux.HandleFunc("/debug/events", trace.Events)
	// Badger also uses expvar which exposes prometheus compatible metrics on /debug/vars
	server.mux.HandleFunc("/debug/vars", expvar.Handler().ServeHTTP)

	server.mux.HandleFunc("/healthz", healthHandler())
	server.mux.Handle("/metrics", promhttp.Handler())
	server.mux.HandleFunc("/", indexHandler(config))

	addr := fmt.Sprintf(":%v", config.Port)

	h := &http.Server{
		Addr:     addr,
		Handler:  traceWrapper(glogWrapper(server)),
		ErrorLog: log.New(os.Stdout, "http: ", log.LstdFlags),
	}

	glog.Infof("Listening on http://localhost%v", addr)

	stop := make(chan os.Signal, 1)

	go func() { _ = h.ListenAndServe() }()

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	glog.Infof("Shutting down server...")
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()
	err := h.Shutdown(ctx)
	glog.Infof("WebServer closed")

	return err
}
