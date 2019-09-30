/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package badgerwrap

import (
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
)

type BadgerFactory struct {
}

type BadgerDb struct {
	db *badger.DB
}

type BadgerTxn struct {
	txn *badger.Txn
}

type BadgerItem struct {
	item *badger.Item
}

type BadgerIterator struct {
	itr *badger.Iterator
}

func (f *BadgerFactory) Open(opt badger.Options) (DB, error) {
	db, err := badger.Open(opt)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open badger")
	}
	return &BadgerDb{db: db}, nil
}

// Database

func (b *BadgerDb) Close() error {
	return b.db.Close()
}

func (b *BadgerDb) Sync() error {
	return b.db.Sync()
}

func (b *BadgerDb) Update(fn func(txn Txn) error) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return fn(&BadgerTxn{txn: txn})
	})
}

func (b *BadgerDb) View(fn func(txn Txn) error) error {
	return b.db.View(func(txn *badger.Txn) error {
		return fn(&BadgerTxn{txn: txn})
	})
}

func (b *BadgerDb) DropPrefix(prefix []byte) error {
	err := b.db.DropPrefix(prefix)
	return err
}

func (b *BadgerDb) Size() (lsm, vlog int64) {
	return b.db.Size()
}

func (b *BadgerDb) Tables(withKeysCount bool) []badger.TableInfo {
	return b.db.Tables(withKeysCount)
}

// Transaction

func (t *BadgerTxn) Get(key []byte) (Item, error) {
	item, err := t.txn.Get(key)
	if err != nil {
		return nil, err
	}
	return &BadgerItem{item: item}, nil
}

func (t *BadgerTxn) Set(key, val []byte) error {
	return t.txn.Set(key, val)
}

func (t *BadgerTxn) Delete(key []byte) error {
	return t.txn.Delete(key)
}

func (t *BadgerTxn) NewIterator(opt badger.IteratorOptions) Iterator {
	return &BadgerIterator{itr: t.txn.NewIterator(opt)}
}

// Item

func (i *BadgerItem) Key() []byte {
	return i.item.Key()
}

func (i *BadgerItem) Value(fn func(val []byte) error) error {
	return i.item.Value(fn)
}

func (i *BadgerItem) ValueCopy(dst []byte) ([]byte, error) {
	return i.item.ValueCopy(dst)
}

// Iterator

func (i *BadgerIterator) Close() {
	i.itr.Close()
}

func (i *BadgerIterator) Item() Item {
	return i.itr.Item()
}

func (i *BadgerIterator) Next() {
	i.itr.Next()
}

func (i *BadgerIterator) Seek(key []byte) {
	i.itr.Seek(key)
}

func (i *BadgerIterator) Valid() bool {
	return i.itr.Valid()
}

func (i *BadgerIterator) ValidForPrefix(prefix []byte) bool {
	return i.itr.ValidForPrefix(prefix)
}

func (i *BadgerIterator) Rewind() {
	i.itr.Rewind()
}
