/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package ingress

import (
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"io/ioutil"
	"sync"
)

type FileRecorder struct {
	inChan   chan typed.KubeWatchResult
	data     []typed.KubeWatchResult
	filename string
	wg       sync.WaitGroup // Ensure we don't call close at the same time we are taking in events
}

func NewFileRecorder(filename string, inChan chan typed.KubeWatchResult) *FileRecorder {
	fr := &FileRecorder{filename: filename, inChan: inChan}
	return fr
}

func (fr *FileRecorder) Start() {
	fr.wg.Add(1)
	go fr.listen(fr.inChan)
}

func (fr *FileRecorder) listen(inChan chan typed.KubeWatchResult) {
	for {
		newRecord, more := <-inChan
		if !more {
			fr.wg.Done()
			return
		}
		fr.data = append(fr.data, newRecord)
	}
}

func (fr *FileRecorder) Close() error {
	fr.wg.Wait()
	f := KubePlaybackFile{Data: fr.data}
	byteData, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fr.filename, byteData, 0755)
	glog.Infof("Wrote %v records to %v. err %v", len(fr.data), fr.filename, err)
	return err
}
