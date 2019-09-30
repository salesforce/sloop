/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package assertex

import (
	"fmt"
	"github.com/nsf/jsondiff"
	"testing"
)

// This is similar functionality to assert.JSONEq() but is more useful for a couple reasons
// 1) It prints the full actual string without transformed line-breaks, so its easy to copy the output back into source
//    code if desired
// 2) JSONEq prints a diff of the json strings, where this shows a combined diff
//
// Expected Payload:
// {
//   "foo": { "bar": 5 },
//   "abc": [2,3]
// }
//
// Actual Payload
// {
//    "foo": { "bar": 1 },
//    "abc": [2,3]
// }
//
// assert.JSONEq() will give you:

/*
	Error Trace:
	Error:		Not equal: map[string]interface {}{"foo":map[string]interface {}{"bar":5}, "abc":[]interface {}{2, 3}} (expected)
			        != map[string]interface {}{"foo":map[string]interface {}{"bar":1}, "abc":[]interface {}{2, 3}} (actual)

			Diff:
			--- Expected
			+++ Actual
			@@ -6,3 +6,3 @@
			  (string) (len=3) "foo": (map[string]interface {}) (len=1) {
			-  (string) (len=3) "bar": (float64) 5
			+  (string) (len=3) "bar": (float64) 1
			  }
*/

// This helper will give you:

/*
 Diff:NoMatch
## EXPECTED:
{
 "foo": { "bar": 5 },
 "abc": [2,3]
}
## ACTUAL:
{
 "foo": { "bar": 1 },
 "abc": [2,3]
}
## DIFF:
{
"abc": [
2,
3
],
"foo": {
"bar": 5 => 1
}
*/

func JsonEqualBytes(t *testing.T, expectedByte []byte, actualByte []byte) {
	diff, diffString := jsondiff.Compare(expectedByte, actualByte, &jsondiff.Options{})
	if diff != jsondiff.FullMatch {
		fmt.Printf("Diff:%v\n", diff.String())
		fmt.Printf("## EXPECTED:\n%v\n", string(expectedByte))
		fmt.Printf("## ACTUAL:\n%v\n", string(actualByte))
		fmt.Printf("## DIFF:\n%v", diffString)
		t.Fail()
	}
}

func JsonEqual(t *testing.T, expectedStr string, actualStr string) {
	JsonEqualBytes(t, []byte(expectedStr), []byte(actualStr))
}
