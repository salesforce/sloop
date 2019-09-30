/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package e2e

import (
	"fmt"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"strings"
	"testing"
	"time"
)

var Payload = `{
   "metadata": {
      "name": "some-name",
      "namespace": "some-namespace",
      "uid": "f8f372a3-f731-11e8-b3bd-e24c7f08fac6",
      "creationTimestamp": "2018-12-03T19:31:03Z"
   }
}
`

var SimpleQueryPlayback = fmt.Sprintf(`Data:
- kind: Deployment
  payload: '%v'
  timestamp:
    nanos: 557590245
    seconds: 1562963506`, strings.ReplaceAll(Payload, "\n", ""))

const SimpleQueryExpected = `{
 "view_options": {
  "sort": ""
 },
 "rows": [
  {
   "text": "some-name",
   "duration": 1209600,
   "kind": "Deployment",
   "namespace": "some-namespace",
   "overlays": [],
   "changedat": null,
   "nochangeat": [
    1562963506
   ],
   "start_date": 1561755600,
   "end_date": 1562965200
  }
 ]
}`

// This test exercises main() with a sample input and compares the result query
// These should be used sparingly, and most query tests should be done in the query folder
func Test_SimpleQueryWithOneResource(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	helper_runE2E([]byte(SimpleQueryPlayback), []byte(SimpleQueryExpected), "EventHeatMap", t)
}
