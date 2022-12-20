/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package assertex

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
)

func areProtoEqual(expected interface{}, actual interface{}) bool {
	expectedProto, ok := expected.(proto.Message)
	if ok {
		actualProto, ok := actual.(proto.Message)
		if ok {
			return proto.Equal(expectedProto, actualProto)
		}
	}
	return false
}

func ProtoEqual(t *testing.T, expected interface{}, actual interface{}) {
	if !areProtoEqual(expected, actual) {
		fmt.Printf("## EXPECTED:\n%v\n", expected)
		fmt.Printf("## ACTUAL:\n%v\n", actual)
		t.Fail()
	}
}
