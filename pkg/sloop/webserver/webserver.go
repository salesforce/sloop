/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"github.com/salesforce/sloop/pkg/sloop/queries"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"log"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	debugViewKeyTemplateFile  = "debugviewkey.html"
	debugListKeysTemplateFile = "debuglistkeys.html"
	debugConfigTemplateFile   = "debugconfig.html"
	indexTemplateFile         = "index.html"
	resourceTemplateFile      = "resource.html"
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
	metricWebServerRequestCount = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_webserver_request_count"})
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

func Run(config WebConfig, tables typed.Tables) error {
	webFiles = config.WebFilesPath
	server := &Server{}
	server.mux = http.NewServeMux()
	server.mux.HandleFunc("/webfiles/", webFileHandler)
	server.mux.HandleFunc("/data", queryHandler(tables, config.MaxLookback))
	server.mux.HandleFunc("/resource", resourceHandler(config.ResourceLinks))
	server.mux.HandleFunc("/debug/", listKeysHandler(tables))
	server.mux.HandleFunc("/debug/view/", viewKeyHandler(tables))
	server.mux.HandleFunc("/debug/config/", configHandler(config.ConfigYaml))
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
