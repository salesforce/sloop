/**
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
)

func PlayFile(outChan chan typed.KubeWatchResult, filename string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	var playbackFile KubePlaybackFile
	err = yaml.Unmarshal(b, &playbackFile)
	if err != nil {
		return err
	}

	glog.Infof("Loaded %v resources from file source %v", len(playbackFile.Data), filename)

	for _, watchRecord := range playbackFile.Data {
		outChan <- watchRecord
	}
	glog.Infof("Done writing kubeWatch events to channel")
	return nil
}
