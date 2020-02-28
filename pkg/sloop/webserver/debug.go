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
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/common"
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

type sloopKeyInfo struct {
	MinimumSize int64
	MaximumSize int64
	TotalKeys   int64
	TotalSize   int64
	AverageSize int64
}

type sloopKey struct {
	TableName   string
	PartitionID string
}

type histogram struct {
	HistogramMap map[sloopKey]*sloopKeyInfo
	TotalKeys    int
	DeletedKeys  int
}

// returns TableName, PartitionId, error.
func parseSloopKey(item badgerwrap.Item) (string, string, error) {
	key := item.Key()
	err, parts := common.ParseKey(string(key))
	if err != nil {
		return "", "", err
	}

	var tableName = parts[1]
	var partitionId = parts[2]
	return tableName, partitionId, nil
}

func histogramHandler(tables typed.Tables) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var result histogram
		prefix := request.URL.Query().Get("prefix")
		if len(prefix) > 0 {

			if prefix == "*" {
				prefix = ""
			}

			err := tables.Db().View(func(txn badgerwrap.Txn) error {
				iterOpt := badger.DefaultIteratorOptions
				iterOpt.Prefix = []byte(prefix)
				iterOpt.PrefetchValues = false
				itr := txn.NewIterator(iterOpt)
				defer itr.Close()

				totalKeys := 0
				totalDeletedExpiredKeys := 0
				var sloopMap = make(map[sloopKey]*sloopKeyInfo)
				for itr.Rewind(); itr.Valid(); itr.Next() {
					item := itr.Item()
					tableName, partitionId, err := parseSloopKey(item)
					if err != nil {
						return errors.Wrapf(err, "failed to parse information about key: %x",
							item.Key())
					}
					totalKeys++

					if item.IsDeletedOrExpired() {
						totalDeletedExpiredKeys++
					}

					size := item.EstimatedSize()
					sloopKey := sloopKey{tableName, partitionId}
					if sloopMap[sloopKey] == nil {
						sloopMap[sloopKey] = &sloopKeyInfo{size, size, 1, size, size}
					} else {
						sloopMap[sloopKey].TotalKeys++
						sloopMap[sloopKey].TotalSize += size
						sloopMap[sloopKey].AverageSize = sloopMap[sloopKey].TotalSize / sloopMap[sloopKey].TotalKeys
						if size < sloopMap[sloopKey].MinimumSize {
							sloopMap[sloopKey].MinimumSize = size
						}

						if size > sloopMap[sloopKey].MaximumSize {
							sloopMap[sloopKey].MaximumSize = size
						}
					}
				}

				result.TotalKeys = totalKeys
				result.DeletedKeys = totalDeletedExpiredKeys
				result.HistogramMap = sloopMap
				return nil
			})

			if err != nil {
				logWebError(err, "Could not get histogram", request, writer)
				return
			}
		}
		writer.Header().Set("content-type", "text/html")

		t, err := template.New(debugHistogramFile).ParseFiles(path.Join(webFiles, debugHistogramFile))
		if err != nil {
			logWebError(err, "failed to parse histogram template", request, writer)
			return
		}

		err = t.ExecuteTemplate(writer, debugHistogramFile, result)
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

func debugHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		t, err := template.New(debugTemplateFile).ParseFiles(path.Join(webFiles, debugTemplateFile))
		if err != nil {
			logWebError(err, "failed to parse template", request, writer)
			return
		}
		err = t.ExecuteTemplate(writer, debugTemplateFile, nil)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}

// Make a copy with string keys instead of []byte keys
type badgerTableInfo struct {
	Level    int
	LeftKey  string
	RightKey string
	KeyCount uint64
	ID       uint64
}

func debugBadgerTablesHandler(db badgerwrap.DB) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		t, err := template.New(debugBadgerTablesTemplateFile).ParseFiles(path.Join(webFiles, debugBadgerTablesTemplateFile))
		if err != nil {
			logWebError(err, "failed to parse template", request, writer)
			return
		}
		data := []badgerTableInfo{}
		for _, table := range db.Tables(true) {
			thisTable := badgerTableInfo{
				Level:    table.Level,
				LeftKey:  string(table.Left),
				RightKey: string(table.Right),
				KeyCount: table.KeyCount,
				ID:       table.ID,
			}
			data = append(data, thisTable)
		}
		err = t.ExecuteTemplate(writer, debugBadgerTablesTemplateFile, data)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}
