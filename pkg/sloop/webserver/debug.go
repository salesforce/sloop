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
	"regexp"
	"strings"
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

		//t, err := template.New(debugViewKeyTemplateFile).ParseFiles(path.Join(webFiles, debugViewKeyTemplateFile))
		//if err != nil {
		//	logWebError(err, "failed to parse template", request, writer)
		//	return
		//}
		debugviewkeyTemplate, err := GetTemplate("debugviewkey.html")
		if err != nil {
			logWebError(err, "failed to parse template", request, writer)
			return
		}
		err = debugviewkeyTemplate.Execute(writer, data)
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
		totalCount := 0
		var totalSize int64 = 0
		err = tables.Db().View(func(txn badgerwrap.Txn) error {
			keyPrefix := ""
			if table != "internal" {
				keyPrefix = "/" + table + "/"
			}

			iterOpt := badger.DefaultIteratorOptions
			iterOpt.Prefix = []byte(keyPrefix)
			iterOpt.AllVersions = true
			iterOpt.InternalAccess = true
			itr := txn.NewIterator(iterOpt)
			defer itr.Close()

			// TODO: Investigate if Seek() can be used instead of rewind
			for itr.Rewind(); itr.ValidForPrefix([]byte(keyPrefix)); itr.Next() {
				totalCount++
				thisKey := string(itr.Item().Key())
				if keyRegEx.MatchString(thisKey) {
					keys = append(keys, thisKey)
					count += 1
					totalSize += itr.Item().EstimatedSize()
					if count >= maxRows {
						glog.Infof("Number of rows : %v has reached max rows: %v", count, maxRows)
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

		debugListKeysTemplate, err := GetTemplate("debuglistkeys.html")
		if err != nil {
			logWebError(err, "failed to parse template", request, writer)
			return
		}
		var result keysData
		result.Keys = keys
		result.TotalKeys = totalCount
		result.TotalSize = totalSize
		result.KeysMatched = count
		err = debugListKeysTemplate.Execute(writer, result)
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

type histogram struct {
	HistogramMap          map[common.SloopKey]*sloopKeyInfo
	TotalKeys             int
	TotalSloopKeys        int
	TotalEstimatedSize    int64
	DeletedKeys           int
	TotalInternalKeys     int
	TotalInternalKeysSize int64
	TotalHeadKeys         int
	TotalMoveKeys         int
	TotalDiscardKeys      int
}

type keysData struct {
	Keys        []string
	TotalKeys   int
	TotalSize   int64
	KeysMatched int
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
				iterOpt.AllVersions = true
				iterOpt.InternalAccess = true
				itr := txn.NewIterator(iterOpt)
				defer itr.Close()

				totalKeys := 0
				var totalEstimatedSize int64 = 0
				var totalInternalKeysSize int64 = 0
				totalDeletedExpiredKeys := 0
				totalInternalKeys := 0
				totalMoveKeys := 0
				totalHeadKeys := 0
				totalDiscardKeys := 0
				totalSloopKeys := 0
				var sloopMap = make(map[common.SloopKey]*sloopKeyInfo)
				for itr.Rewind(); itr.Valid(); itr.Next() {
					item := itr.Item()
					size := item.EstimatedSize()
					totalEstimatedSize += size
					totalKeys++
					if item.IsDeletedOrExpired() {
						totalDeletedExpiredKeys++
					}

					if strings.HasPrefix(string(item.Key()), "!badger") {
						totalInternalKeys++
						totalInternalKeysSize += item.EstimatedSize()
						if strings.HasPrefix(string(item.Key()), "!badger!head") {
							totalHeadKeys++
						} else if strings.HasPrefix(string(item.Key()), "!badger!move") {
							totalMoveKeys++
						} else if strings.HasPrefix(string(item.Key()), "!badger!discard") {
							totalDiscardKeys++
						}
					} else {
						totalSloopKeys++
						sloopKey, err := common.GetSloopKey(item)
						if err != nil {
							return errors.Wrapf(err, "failed to parse information about key: %x",
								item.Key())
						}

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
				}

				result.TotalKeys = totalKeys
				result.DeletedKeys = totalDeletedExpiredKeys
				result.HistogramMap = sloopMap
				result.TotalDiscardKeys = totalDiscardKeys
				result.TotalEstimatedSize = totalEstimatedSize
				result.TotalHeadKeys = totalHeadKeys
				result.TotalInternalKeys = totalInternalKeys
				result.TotalMoveKeys = totalMoveKeys
				result.TotalInternalKeysSize = totalInternalKeysSize
				result.TotalSloopKeys = totalSloopKeys
				return nil
			})

			if err != nil {
				logWebError(err, "Could not get histogram", request, writer)
				return
			}
		}
		writer.Header().Set("content-type", "text/html")

		debugHistogramTemplate, err := GetTemplate("debughistogram.html")

		err = debugHistogramTemplate.Execute(writer, result)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}

func configHandler(config string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		debugConfigTemplate, err := GetTemplate("debugconfig.html")
		err = debugConfigTemplate.Execute(writer, config)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}

func debugHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		debugTemplate, err := GetTemplate("debug.html")
		err = debugTemplate.Execute(writer,nil)
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
	Size     uint64
}

func debugBadgerTablesHandler(db badgerwrap.DB) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		debugBadgerTablesTemplate, err := GetTemplate("debugtables.html")
		data := []badgerTableInfo{}
		for _, table := range db.Tables(true) {
			thisTable := badgerTableInfo{
				Level:    table.Level,
				LeftKey:  string(table.Left),
				RightKey: string(table.Right),
				KeyCount: table.KeyCount,
				ID:       table.ID,
				Size:     table.EstimatedSz,
			}
			data = append(data, thisTable)
		}
		err = debugBadgerTablesTemplate.Execute(writer, data)
		if err != nil {
			logWebError(err, "Template.ExecuteTemplate failed", request, writer)
			return
		}
	}
}
