/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package badgerwrap

import (
	"io"

	"github.com/dgraph-io/badger/v2"
)

// Need a factory we can pass into untyped store so it can open and close databases
// with the proper impl
type Factory interface {
	Open(opt badger.Options) (DB, error)
}

type DB interface {
	Close() error
	Sync() error
	Update(fn func(txn Txn) error) error
	View(fn func(txn Txn) error) error
	DropPrefix(prefix []byte) error
	Size() (lsm, vlog int64)
	Tables(withKeysCount bool) []badger.TableInfo
	Backup(w io.Writer, since uint64) (uint64, error)
	//	DropAll() error
	//	Flatten(workers int) error
	//	GetMergeOperator(key []byte, f MergeFunc, dur time.Duration) *MergeOperator
	//	GetSequence(key []byte, bandwidth uint64) (*Sequence, error)
	//	KeySplits(prefix []byte) []string
	Load(r io.Reader, maxPendingWrites int) error
	//	MaxBatchCount() int64
	//	MaxBatchSize() int64
	//	NewKVLoader(maxPendingWrites int) *KVLoader
	//	NewStream() *Stream
	//	NewStreamAt(readTs uint64) *Stream
	//	NewStreamWriter() *StreamWriter
	//	NewTransaction(update bool) *Txn
	//	NewTransactionAt(readTs uint64, update bool) *Txn
	//	NewWriteBatch() *WriteBatch
	//	PrintHistogram(keyPrefix []byte)
	RunValueLogGC(discardRatio float64) error
	//	SetDiscardTs(ts uint64)
	//	Subscribe(ctx context.Context, cb func(kv *KVList), prefixes ...[]byte) error
	//	VerifyChecksum() error
}

type Txn interface {
	Get(key []byte) (Item, error)
	Set(key, val []byte) error
	Delete(key []byte) error
	NewIterator(opt badger.IteratorOptions) Iterator
	//  NewKeyIterator(key []byte, opt badger.IteratorOptions) *badger.Iterator
	//	ReadTs() uint64
	//	SetEntry(e *badger.Entry) error
	//	Discard()
	//	Commit() error
	//	CommitAt(commitTs uint64, callback func(error)) error
	//	CommitWith(cb func(error))
}

type Item interface {
	Key() []byte
	Value(fn func(val []byte) error) error
	ValueCopy(dst []byte) ([]byte, error)
	//	DiscardEarlierVersions() bool
	EstimatedSize() int64
	//	ExpiresAt() uint64
	IsDeletedOrExpired() bool
	KeyCopy(dst []byte) []byte
	//	KeySize() int64
	//	String() string
	//	UserMeta() byte
	//	ValueSize() int64
	//	Version() uint64
}

type Iterator interface {
	Close()
	Item() Item
	Next()
	Seek(key []byte)
	Valid() bool
	ValidForPrefix(prefix []byte) bool
	Rewind()
}
