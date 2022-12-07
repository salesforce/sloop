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

	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/spf13/afero"

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
	BindAddress      string
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
	metricWebServerRequestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sloop_webserver_http_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "handler"},
	)
	metricWebServerRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sloop_webserver_http_request_duration_seconds",
			Help:    "A histogram of latencies for requests to the wrapped handler.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"handler"},
	)
)

// This is not going to change and we don't want to pass it to every function
// so use a static for now
var webFilesPath string

func logWebError(err error, note string, r *http.Request, w http.ResponseWriter) {
	message := fmt.Sprintf("Error rendering url: %q.  Note: %v. Error: %v", r.URL, note, err)
	glog.ErrorDepth(1, message)
	http.Error(w, message, http.StatusInternalServerError)
}

// TODO: We should probably only allow a fixed set of files to read so users don't get creative

// Example input: r.URL=/webfiles/static/style.css
// Returns file: <webFiles>/static/style.css
func webFileHandler(currentContext string) http.HandlerFunc {
	webFilesPathToTrim := path.Join("/", currentContext, "/webfiles")
	return func(w http.ResponseWriter, r *http.Request) {
		fixedUrl := strings.TrimPrefix(fmt.Sprint(r.URL), webFilesPathToTrim)
		if strings.Contains(fixedUrl, "..") {
			logWebError(nil, "Not allowed", r, w)
			return
		}
		data, err := readWebfile(fixedUrl, &afero.Afero{afero.NewOsFs()})
		if err != nil {
			logWebError(err, "Error reading web file: "+fixedUrl, r, w)
			return
		}
		fullPath := common.GetFilePath(prefix, fixedUrl)
		w.Header().Set("content-type", mime.TypeByExtension(filepath.Ext(fullPath)))
		_, err = w.Write(data)
		if err != nil {
			logWebError(err, "Error writing web file: "+fixedUrl, r, w)
			return
		}
		glog.V(2).Infof("webFileHandler successfully returned file %v for %v", fixedUrl, r.URL)
	}
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

// Handler for redirecting / to /currentContext to ensure backward compatibility
func redirectHandler(currentContext string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		redirectURL := path.Join("/", currentContext, request.URL.Path)
		http.Redirect(writer, request, redirectURL, http.StatusTemporaryRedirect)
	}
}

// Registers routes on mux router
func registerRoutes(mux *http.ServeMux, config WebConfig, tables typed.Tables) {
	// root pages
	mux.Handle("/", wrapperChain("root", redirectHandler(config.CurrentContext)))
	mux.Handle("/healthz", metricCountsWrapper("healthz", healthHandler()))
	mux.Handle("/metrics", metricCountsWrapper("metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	)))

	// /<currentContext> pages
	ccPrefix := fmt.Sprintf("/%s", config.CurrentContext)
	mux.HandleFunc(ccPrefix, wrapperChain("index", indexHandler(config)))
	mux.HandleFunc(ccPrefix+"/webfiles/", wrapperChain("webFile", webFileHandler(config.CurrentContext)))
	mux.HandleFunc(ccPrefix+"/data/backup", wrapperChain("backup", backupHandler(tables.Db(), config.CurrentContext)))
	mux.HandleFunc(ccPrefix+"/data", wrapperChain("query", queryHandler(tables, config.MaxLookback)))
	mux.HandleFunc(ccPrefix+"/resource", wrapperChain("resource", resourceHandler(config.ResourceLinks, config.CurrentContext)))
	// Debug pages
	mux.HandleFunc(ccPrefix+"/debug/listkeys/", wrapperChain("debug", listKeysHandler(tables)))
	mux.HandleFunc(ccPrefix+"/debug/histogram/", wrapperChain("debug", histogramHandler(tables)))
	mux.HandleFunc(ccPrefix+"/debug/tables/", wrapperChain("debug", debugBadgerTablesHandler(tables.Db())))
	mux.HandleFunc(ccPrefix+"/debug/view", wrapperChain("debug", viewKeyHandler(tables)))
	mux.HandleFunc(ccPrefix+"/debug/config/", wrapperChain("debug", configHandler(config.ConfigYaml)))
	// Badger uses the trace package, which registers /debug/requests and /debug/events
	mux.HandleFunc(ccPrefix+"/debug/requests", wrapperChain("debug", http.HandlerFunc(trace.Traces)))
	mux.HandleFunc(ccPrefix+"/debug/events", wrapperChain("debug", http.HandlerFunc(trace.Events)))
	// Badger also uses expvar which exposes prometheus compatible metrics on /debug/vars
	mux.HandleFunc(ccPrefix+"/debug/vars", wrapperChain("debug", http.HandlerFunc(expvar.Handler().ServeHTTP)))
	mux.HandleFunc(ccPrefix+"/debug/", wrapperChain("debug", debugHandler()))
}

func Run(config WebConfig, tables typed.Tables) error {
	webFilesPath = config.WebFilesPath

	mux := http.NewServeMux()
	registerRoutes(mux, config, tables)

	addr := fmt.Sprintf("%v:%v", config.BindAddress, config.Port)

	h := &http.Server{
		Addr:     addr,
		Handler:  mux,
		ErrorLog: log.New(os.Stdout, "http: ", log.LstdFlags),
	}
	if config.BindAddress != "" {
		glog.Infof("Listening on http://%v", addr)
	} else {
		glog.Infof("Listening on http://localhost%v", addr)
	}

	stop := make(chan os.Signal, 1)

	go func() {
		err := h.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			glog.Fatal(err)
		}
	}()

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	glog.Infof("Shutting down server...")
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()
	err := h.Shutdown(ctx)
	glog.Infof("WebServer closed")

	return err
}
