/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"html/template"
	"net/http"
	"path"
)

type indexData struct {
	DefaultLookback    string
	DefaultNamespace   string
	DefaultKind        string
	DefaultBucketWidth string
	LeftBarLinks       []ComputedLink
	CurrentContext     string
}

func indexHandler(config WebConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		t, err := template.New(indexTemplateFile).ParseFiles(path.Join(webFiles, indexTemplateFile))
		if err != nil {
			logWebError(err, "Template.New failed", request, writer)
			return
		}

		data := indexData{}
		data.DefaultLookback = config.DefaultLookback
		data.DefaultNamespace = config.DefaultNamespace
		data.DefaultKind = config.DefaultResources
		data.DefaultBucketWidth = config.DefaultBucketWidth
		data.CurrentContext = config.CurrentContext
		data.LeftBarLinks, err = makeLeftBarLinks(config.LeftBarLinks)
		if err != nil {
			logWebError(err, "Could not make left bar links", request, writer)
			return
		}

		err = t.ExecuteTemplate(writer, indexTemplateFile, data)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}
