package typed

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

type RelationshipTable struct {
	tableName string
}

func OpenRelationshipTable() *RelationshipTable {
	keyInst := &RelationshipKey{}
	return &RelationshipTable{tableName: keyInst.TableName()}
}

//todo: create Relationship value
func (t *RelationshipTable) Set(txn badgerwrap.Txn, key string, value *Relationship) error {
	err := (&RelationshipKey{}).ValidateKey(key)
	if err != nil {
		return errors.Wrapf(err, "invalid key for table %v: %v", t.tableName, key)
	}

	outb, err := proto.Marshal(value)
	if err != nil {
		return errors.Wrapf(err, "protobuf marshal for table %v failed", t.tableName)
	}

	err = txn.Set([]byte(key), outb)
	if err != nil {
		return errors.Wrapf(err, "set for table %v failed", t.tableName)
	}
	return nil
}

//unmarshal files to object
func (t *RelationshipTable) Get(txn badgerwrap.Txn, key string) (*Relationship, error) {
	err := (&RelationshipKey{}).ValidateKey(key)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid key for table %v: %v", t.tableName, key)
	}

	item, err := txn.Get([]byte(key))
	if err == badger.ErrKeyNotFound {
		// Dont wrap. Need to preserve error type
		return nil, err
	} else if err != nil {
		return nil, errors.Wrapf(err, "get failed for table %v", t.tableName)
	}

	valueBytes, err := item.ValueCopy([]byte{})
	if err != nil {
		return nil, errors.Wrapf(err, "value copy failed for table %v", t.tableName)
	}

	retValue := &Relationship{}
	err = proto.Unmarshal(valueBytes, retValue)
	if err != nil {
		return nil, errors.Wrapf(err, "protobuf unmarshal failed for table %v on value length %v", t.tableName, len(valueBytes))
	}
	return retValue, nil
}

