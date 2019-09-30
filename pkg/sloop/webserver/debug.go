/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"html/template"
	"net/http"
	"path"
	"regexp"
)

type keyView struct {
	Key        string
	Payload    template.HTML
	ExtraName  string
	ExtraValue template.HTML
}

// DEBUG PAGES
// todo: move these to a new file when we make a webserver directory in later PR

func jsonPrettyPrint(in string) string {
	var out bytes.Buffer
	err := json.Indent(&out, []byte(in), "", "  ")
	if err != nil {
		return in
	}
	return out.String()
}

func viewKeyHandler(tables typed.Tables) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("content-type", "text/html")

		key := request.FormValue("k")
		data := keyView{}
		data.Key = key

		var valueFromTable interface{}

		err := tables.Db().View(func(txn badgerwrap.Txn) error {
			if (&typed.WatchTableKey{}).ValidateKey(key) == nil {
				kwr, err := tables.WatchTable().Get(txn, key)
				if err != nil {
					return err
				}
				valueFromTable = *kwr
				data.ExtraName = "$.Payload"
				data.ExtraValue = template.HTML(jsonPrettyPrint(kwr.Payload))
			} else if (&typed.ResourceSummaryKey{}).ValidateKey(key) == nil {
				rs, err := tables.ResourceSummaryTable().Get(txn, key)
				if err != nil {
					return err
				}
				valueFromTable = *rs
			} else if (&typed.EventCountKey{}).ValidateKey(key) == nil {
				ec, err := tables.EventCountTable().Get(txn, key)
				if err != nil {
					return err
				}
				valueFromTable = *ec
			} else if (&typed.WatchActivityKey{}).ValidateKey(key) == nil {
				wa, err := tables.WatchActivityTable().Get(txn, key)
				if err != nil {
					return err
				}
				valueFromTable = *wa
			} else {
				return fmt.Errorf("Invalid key: %v", key)
			}
			return nil
		})
		if err != nil {
			logWebError(err, "view transaction failed", request, writer)
			return
		}

		prettyJson, err := json.MarshalIndent(valueFromTable, "", "  ")
		if err != nil {
			logWebError(err, fmt.Sprintf("Failed to marshal kubeWatchResult for key: %v", key), request, writer)
			return
		}
		data.Payload = template.HTML(string(prettyJson))

		t, err := template.New(debugViewKeyTemplateFile).ParseFiles(path.Join(webFiles, debugViewKeyTemplateFile))
		if err != nil {
			logWebError(err, "failed to parse template", request, writer)
			return
		}

		err = t.ExecuteTemplate(writer, debugViewKeyTemplateFile, data)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}

func listKeysHandler(tables typed.Tables) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {

		table := cleanStringFromParam(request, "table", "")
		keyMatchRegExStr := request.URL.Query().Get("keymatch")
		keyRegEx, err := regexp.Compile(keyMatchRegExStr)
		if err != nil {
			logWebError(err, "Invalid regex", request, writer)
		}
		maxRows := numberFromParam(request, "maxrows", 500)
		var keys []string

		count := 0
		err = tables.Db().View(func(txn badgerwrap.Txn) error {
			keyPrefix := "/" + table + "/"
			iterOpt := badger.DefaultIteratorOptions
			iterOpt.Prefix = []byte(keyPrefix)
			itr := txn.NewIterator(iterOpt)
			defer itr.Close()

			for itr.Seek([]byte(keyPrefix)); itr.ValidForPrefix([]byte(keyPrefix)); itr.Next() {
				thisKey := string(itr.Item().Key())
				if keyRegEx.MatchString(thisKey) {
					keys = append(keys, thisKey)
					count += 1
					if count >= maxRows {
						glog.Infof("Reached max rows: %v", maxRows)
						break
					}
				}
			}
			return nil
		})
		if err != nil {
			logWebError(err, "Could not list keys", request, writer)
			return
		}

		writer.Header().Set("content-type", "text/html")

		t, err := template.New(debugListKeysTemplateFile).ParseFiles(path.Join(webFiles, debugListKeysTemplateFile))
		if err != nil {
			logWebError(err, "failed to parse template", request, writer)
			return
		}

		err = t.ExecuteTemplate(writer, debugListKeysTemplateFile, keys)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}

func configHandler(config string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		t, err := template.New(debugConfigTemplateFile).ParseFiles(path.Join(webFiles, debugConfigTemplateFile))
		if err != nil {
			logWebError(err, "failed to parse template", request, writer)
			return
		}
		err = t.ExecuteTemplate(writer, debugConfigTemplateFile, config)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}
