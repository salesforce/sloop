package typed

import (
	"fmt"
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"time"
)
// Relationship table to keep node/rs to pods mapping

// Key is /<partition>/<kind>/<namespace>/<name>/<uid>
//
// Partition is UnixSeconds rounded down to partition duration
// Kind is kubernetes kind, starts with upper case
// Namespace is kubernetes namespace, all lower
// Name is kubernetes name, all lower
// Uid is kubernetes $.metadata.uid

type RelationshipKey struct {
	PartitionId string
	Kind        string
	Namespace   string
	Name        string
	Uid         string
}

func (*RelationshipKey) TableName() string {
	return "relation"
}

func NewRelationshipKey(timestamp time.Time, kind string, namespace string, name string, uid string) *RelationshipKey {
	partitionId := untyped.GetPartitionId(timestamp)
	return &RelationshipKey{PartitionId: partitionId, Kind: kind, Namespace: namespace, Name: name, Uid: uid}
}

func NewRelationshipKeyComparator(kind string, namespace string, name string, uid string) *RelationshipKey {
	return &RelationshipKey{Kind: kind, Namespace: namespace, Name: name, Uid: uid}
}

func (k *RelationshipKey) Parse(key string) error {
	err, parts := common.ParseKey(key)
	if err != nil {
		return err
	}

	if parts[1] != k.TableName() {
		return fmt.Errorf("Second part of key (%v) should be %v", key, k.TableName())
	}
	k.PartitionId = parts[2]
	k.Kind = parts[3]
	k.Namespace = parts[4]
	k.Name = parts[5]
	k.Uid = parts[6]
	return nil
}

func (k *RelationshipKey) String() string {
	if k.Uid == "" {
		return fmt.Sprintf("/%v/%v/%v/%v/%v", k.TableName(), k.PartitionId, k.Kind, k.Namespace, k.Name)
	} else {
		return fmt.Sprintf("/%v/%v/%v/%v/%v/%v", k.TableName(), k.PartitionId, k.Kind, k.Namespace, k.Name, k.Uid)
	}
}

func (k *RelationshipKey) SetPartitionId(newPartitionId string) {
	k.PartitionId = newPartitionId
}

func (*RelationshipKey) ValidateKey(key string) error {
	newKey := RelationshipKey{}
	return newKey.Parse(key)
}
