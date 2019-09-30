/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package e2e

import (
	"github.com/salesforce/sloop/pkg/sloop/server"
	"github.com/salesforce/sloop/pkg/sloop/server/internal/config"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
	"time"
)

func helper_runE2E(playbackData []byte, expectedOutput []byte, queryName string, t *testing.T) {
	// Badger Data DB
	dataDir, err := ioutil.TempDir("", "data")
	assert.Nil(t, err)

	// Playback File
	playbackFile, err := ioutil.TempFile("", "playback")
	assert.Nil(t, err)
	_, err = playbackFile.Write([]byte(playbackData))
	assert.Nil(t, err)
	playbackFile.Close()

	// Test config
	testConfig := &config.SloopConfig{}
	testConfig.DisableKubeWatcher = true
	testConfig.DebugDisableWebServer = true
	testConfig.StoreRoot = dataDir
	testConfig.DebugPlaybackFile = playbackFile.Name()
	testConfig.DebugRunQuery = queryName
	testConfig.UseMockBadger = true
	testConfig.DisableStoreManager = true
	testConfig.MaxLookback = 14 * 24 * time.Hour

	outData, err := server.RunWithConfig(testConfig)
	assert.Nil(t, err)
	assertex.JsonEqualBytes(t, []byte(expectedOutput), outData)
}
