/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"
)

func StringToProtobufTimestamp(ts string) (*timestamp.Timestamp, error) {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse timestamp")
	}

	tspb, err := ptypes.TimestampProto(t)
	if err != nil {
		return nil, errors.Wrap(err, "could not transform to proto timestamp")
	}

	return tspb, nil
}
