/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"net/http"
)

type indexData struct {
	DefaultLookback  string
	DefaultNamespace string
	DefaultKind      string
	LeftBarLinks     []ComputedLink
	CurrentContext   string
}

func indexHandler(config WebConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		indexTemplate, err := GetTemplate(indexTemplateFile)
		if err != nil {
			logWebError(err, "Template.New failed", request, writer)
			return
		}
		data := indexData{}
		data.DefaultLookback = config.DefaultLookback
		data.DefaultNamespace = config.DefaultNamespace
		data.DefaultKind = config.DefaultResources
		data.CurrentContext = config.CurrentContext
		data.LeftBarLinks, err = makeLeftBarLinks(config.LeftBarLinks)
		if err != nil {
			logWebError(err, "Could not make left bar links", request, writer)
			return
		}

		err = indexTemplate.Execute(writer, data)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}
