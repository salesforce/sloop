/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package e2e

import (
	"flag"
	"fmt"
)

func init() {
	flag.Set("alsologtostderr", fmt.Sprintf("%t", true))
}
