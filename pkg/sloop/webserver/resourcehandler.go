/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"bytes"
	"fmt"
	"github.com/salesforce/sloop/pkg/sloop/queries"
	"html/template"
	"net/http"
	"path"
	"strings"
	"time"
)

type externalLink struct {
	Text string
	Url  string
}

type resourceData struct {
	Namespace string
	Name      string
	Kind      string
	Uuid      string
	ClickTime time.Time
	SelfUrl   string
	Links     []ComputedLink
	EventsUrl string
}

func runTextTemplate(templateStr string, data interface{}) (string, error) {
	funcMap := template.FuncMap{
		"ToUpper": strings.ToUpper,
		"ToLower": strings.ToLower,
	}
	tmpl, err := template.New("").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	err = tmpl.Execute(&tpl, data)
	if err != nil {
		return "", err
	}
	return tpl.String(), nil
}

func resourceHandler(resLinks []ResourceLinkTemplate) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		t, err := template.New(resourceTemplateFile).ParseFiles(path.Join(webFiles, resourceTemplateFile))
		if err != nil {
			logWebError(err, "Template.New failed", request, writer)
			return
		}

		d := resourceData{}
		d.Namespace = cleanStringFromParam(request, queries.NamespaceParam, "")
		d.Name = cleanStringFromParam(request, queries.NameParam, "")
		d.Kind = cleanStringFromParam(request, queries.KindParam, "")
		d.Uuid = cleanStringFromParam(request, queries.UuidParam, "")
		d.ClickTime, err = timeFromUnixTimeParam(request, queries.ClickTimeParam, time.Time{}, time.Millisecond)
		if err != nil || d.ClickTime == (time.Time{}) {
			logWebError(err, "Invalid click time", request, writer)
			return
		}
		d.SelfUrl = request.URL.String()
		d.Links, err = makeResourceLinks(d.Namespace, d.Name, d.Kind, resLinks)
		if err != nil {
			logWebError(err, "Error creating external links", request, writer)
			return
		}
		dataParams := fmt.Sprintf("?query=%v&namespace=%v&lookback=%v&kind=%v&name=%v", "GetEventData", d.Namespace, "5m", d.Kind, d.Name)
		d.EventsUrl = "/data" + dataParams

		err = t.ExecuteTemplate(writer, resourceTemplateFile, d)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}
