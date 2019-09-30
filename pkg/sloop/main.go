/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/server"
	"os"
	"runtime/pprof"
)

var cpuprofile = flag.String("cpuprofile", "", "write profile to file")

func main() {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			glog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	err := server.RealMain()
	if err != nil {
		glog.Errorf("Main exited with error: %v\n", err)
		os.Exit(1)
	} else {
		glog.Infof("Shutting down gracefully")
		os.Exit(0)
	}
}
