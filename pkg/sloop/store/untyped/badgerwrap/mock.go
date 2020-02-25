/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package badgerwrap

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/dgraph-io/badger/v2"
)

// This mock simulates badger using an in-memory store
// Useful for fast unit tests that don't want to touch the disk
// Currently this uses a crude lock to simulate transactions

type MockFactory struct {
}

type MockDb struct {
	lock *sync.RWMutex
	data map[string][]byte
}

type MockTxn struct {
	readOnly bool
	db       *MockDb
}

type MockItem struct {
	key   []byte
	value []byte
}

type MockIterator struct {
	opt        badger.IteratorOptions
	currentIdx int
	db         *MockDb
	// A snapshot of keys in sorted order
	keys []string
}

func (f *MockFactory) Open(opt badger.Options) (DB, error) {
	return &MockDb{lock: &sync.RWMutex{}, data: make(map[string][]byte)}, nil
}

// Database

func (b *MockDb) Close() error {
	return nil
}

func (b *MockDb) Sync() error {
	return nil
}

func (b *MockDb) Update(fn func(txn Txn) error) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	txn := &MockTxn{readOnly: false, db: b}
	return fn(txn)
}

func (b *MockDb) View(fn func(txn Txn) error) error {
	b.lock.RLock()
	defer b.lock.RUnlock()
	txn := &MockTxn{readOnly: true, db: b}
	return fn(txn)
}

func (b *MockDb) DropPrefix(prefix []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if len(b.data) == 0 {
		return fmt.Errorf("enable to delete prefix: %s from empty table", string(prefix))
	}

	for key, _ := range b.data {
		exists := strings.HasPrefix(key, string(prefix))
		if exists {
			delete(b.data, key)
		}
	}
	return nil
}

func (b *MockDb) Size() (lsm, vlog int64) {
	size := 0
	for k, v := range b.data {
		size += len(k) + len(v)
	}
	return int64(size), 0
}

func (b *MockDb) Tables(withKeysCount bool) []badger.TableInfo {
	keyCount := 0
	if withKeysCount {
		keyCount = len(b.data)
	}
	return []badger.TableInfo{
		{KeyCount: uint64(keyCount)},
	}
}

func (b *MockDb) Backup(w io.Writer, since uint64) (uint64, error) {
	return 0, nil
}

func (b *MockDb) Load(r io.Reader, maxPendingWrites int) error {
	return nil
}

func (b *MockDb) RunValueLogGC(discardRatio float64) error {
	return nil
}

// Transaction

func (t *MockTxn) Get(key []byte) (Item, error) {
	data, ok := t.db.data[string(key)]
	if !ok {
		return nil, badger.ErrKeyNotFound
	}
	item := &MockItem{key: key, value: data}
	return item, nil
}

func (t *MockTxn) Set(key, val []byte) error {
	if t.readOnly {
		return badger.ErrReadOnlyTxn
	}
	t.db.data[string(key)] = val
	return nil
}

func (t *MockTxn) Delete(key []byte) error {
	if t.readOnly {
		return badger.ErrReadOnlyTxn
	}
	delete(t.db.data, string(key))
	return nil
}

func (t *MockTxn) NewIterator(opt badger.IteratorOptions) Iterator {
	keys := []string{}
	for k, _ := range t.db.data {
		keys = append(keys, k)
	}
	if opt.Reverse {
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	} else {
		sort.Strings(keys)
	}
	return &MockIterator{db: t.db, currentIdx: 0, opt: opt, keys: keys}
}

// Item

func (i *MockItem) Key() []byte {
	return i.key
}

func (i *MockItem) Value(fn func(val []byte) error) error {
	return fn(i.value)
}

func (i *MockItem) ValueCopy(dst []byte) ([]byte, error) {
	copy(dst, i.value)
	newcopy := make([]byte, len(i.value))
	copy(newcopy, i.value)
	return newcopy, nil
}

func (i *MockItem) EstimatedSize() int64 {
	return int64(len(i.key) + len(i.value))
}

func (i *MockItem) IsDeletedOrExpired() bool {
	return false
}

func (i *MockItem) KeyCopy(dst []byte) []byte {
	copy(dst, i.key)
	newcopy := make([]byte, len(i.key))
	copy(newcopy, i.key)
	return newcopy
}

// Iterator

func (i *MockIterator) Close() {
}

// Item returns pointer to the current key-value pair. This item is only valid until
// it.Next() gets called.
func (i *MockIterator) Item() Item {
	if i.currentIdx < len(i.keys) {
		thisKey := i.keys[i.currentIdx]
		thisValue := i.db.data[thisKey]
		return &MockItem{key: []byte(thisKey), value: thisValue}
	}
	return nil
}

// Next would advance the iterator by one. Always check it.Valid() after a Next() to
// ensure you have access to a valid it.Item().
func (i *MockIterator) Next() {
	i.currentIdx += 1
}

// Seek would seek to the provided key if present. If absent, it would seek to the next
// smallest key greater than the provided key if iterating in the forward direction. Behavior
// would be reversed if iterating backwards.
func (i *MockIterator) Seek(key []byte) {
	if !i.opt.Reverse {
		i.currentIdx = sort.SearchStrings(i.keys, string(key))
	} else {
		// Badger has a silly behavior where everything in the iterator works properly in reverse except Seek
		// I would expect seek in reverse to find the end of the key range based on the prefix but it does not
		// Also, golang search requires ascending order
		sort.Strings(i.keys)
		i.currentIdx = len(i.keys) - sort.SearchStrings(i.keys, string(key))
		sort.Sort(sort.Reverse(sort.StringSlice(i.keys)))
	}
}

// Valid returns false when iteration is done.
func (i *MockIterator) Valid() bool {
	if i.currentIdx < 0 || i.currentIdx >= len(i.keys) {
		return false
	}
	return true
}

// ValidForPrefix returns false when iteration is done or when the current key is not prefixed
// by the specified prefix.
func (i *MockIterator) ValidForPrefix(prefix []byte) bool {
	if !i.Valid() {
		return false
	}
	return strings.HasPrefix(i.keys[i.currentIdx], string(prefix))
}

// Rewind would rewind the iterator cursor all the way to zero-th position, which would be
// the smallest key if iterating forward, and largest if iterating backward. It does not keep
// track of whether the cursor started with a Seek().
func (i *MockIterator) Rewind() {
	i.currentIdx = 0
}
