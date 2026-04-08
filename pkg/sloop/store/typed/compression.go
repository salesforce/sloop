/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"sync"

	"github.com/klauspost/compress/zstd"
)

var (
	zstdEncoder     *zstd.Encoder
	zstdDecoder     *zstd.Decoder
	zstdInitOnce    sync.Once
	zstdInitErr     error

	// ValueCompressionEnabled controls whether new writes are compressed.
	// Reads always auto-detect compressed vs raw data for backward compatibility.
	ValueCompressionEnabled bool
)

var zstdMagic = [4]byte{0x28, 0xB5, 0x2F, 0xFD}

func initZstd() {
	zstdInitOnce.Do(func() {
		zstdEncoder, zstdInitErr = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
		if zstdInitErr != nil {
			return
		}
		var dec *zstd.Decoder
		dec, zstdInitErr = zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))
		if zstdInitErr != nil {
			return
		}
		zstdDecoder = dec
	})
}

// CompressValue compresses data with zstd if compression is enabled.
// Returns raw data unchanged when compression is disabled.
func CompressValue(data []byte) ([]byte, error) {
	if !ValueCompressionEnabled {
		return data, nil
	}
	initZstd()
	if zstdInitErr != nil {
		return nil, zstdInitErr
	}
	return zstdEncoder.EncodeAll(data, make([]byte, 0, len(data))), nil
}

// DecompressValue auto-detects zstd-compressed data via the magic header
// and decompresses it. Uncompressed data is returned as-is.
func DecompressValue(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return data, nil
	}
	if data[0] != zstdMagic[0] || data[1] != zstdMagic[1] || data[2] != zstdMagic[2] || data[3] != zstdMagic[3] {
		return data, nil
	}
	initZstd()
	if zstdInitErr != nil {
		return nil, zstdInitErr
	}
	return zstdDecoder.DecodeAll(data, nil)
}

// IsCompressed checks if data has the zstd magic header.
func IsCompressed(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return data[0] == zstdMagic[0] && data[1] == zstdMagic[1] && data[2] == zstdMagic[2] && data[3] == zstdMagic[3]
}
