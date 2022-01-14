package processing

import (
	"encoding/json"
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

//  transfer watchRec.Payload to Relationship payload: Convert watchRec to record in relationship table and save key-value pair
func updateRelationshipTable(tables typed.Tables, txn badgerwrap.Txn, watchRec *typed.KubeWatchResult, metadata *kubeextractor.KubeMetadata) error {
	if watchRec.Kind != kubeextractor.PodKind {
		glog.V(common.GlogVerbose).Infof("Skipping relationship table update as kubewatch result is not %v", watchRec.Kind)
		return nil
	}
	// convert node -> pod relationship from watchRec
	// get new relationship record from watch record
	record, err := buildRelationshipRec(watchRec)

	//convert timestamp
	ts, err := ptypes.Timestamp(watchRec.Timestamp)
	if err != nil {
		return errors.Wrapf(err, "could not convert timestamp %v", watchRec.Timestamp)
	}
	// get new key
	key := typed.NewRelationshipKey(ts, watchRec.Kind, metadata.Namespace, metadata.Name, metadata.Uid).String()
	// get existing value for the new key
	value, err := getRelationshipValue(tables, txn, key, metadata, watchRec)
	if err != nil {
		return errors.Wrapf(err, "could not get record for key %v", key)
	}
	// append new record
	value.Relationships = append(value.Relationships, record)
	// save back to DB
	err = tables.RelationshipTable().Set(txn, key, value)
	if err != nil {
		return errors.Wrapf(err, "set for the key %v failed", key)
	}

	return nil
}

// return relationship payload entry, error if any
func buildRelationshipRec(watchRec *typed.KubeWatchResult) (string, error) {
	var internalResource InternalResource
	err := json.Unmarshal([]byte(watchRec.Payload), &internalResource)
	if err != nil {
		glog.Error(err, "could not deserialize watchRec.Payload")
		return "", err
	}
	ts, err := ptypes.Timestamp(watchRec.Timestamp)
	if err != nil {
		return "",errors.Wrap(err, "could not convert timestamp")
	}
	partitionId := untyped.GetPartitionId(ts)
	// build record value from watch record
	var relationship RelationShipPayload
	relationship.Kind = kubeextractor.PodKind
	relationship.Name = internalResource.Metadata.PodName
	relationship.NameSpace = internalResource.Metadata.namespace
	relationship.partitionId = partitionId
	relationship.uid = internalResource.Metadata.uid

	serializedRelationship, err := json.Marshal(relationship)
	if err != nil {
		return "", errors.Wrapf(err, "could not serialize object %v", relationship)
	}

	return string(serializedRelationship), nil

}

//get relationship value: firstSeen, lastSeen, and creation time from watch record
func getRelationshipValue(tables typed.Tables, txn badgerwrap.Txn, key string, metadata *kubeextractor.KubeMetadata, rec *typed.KubeWatchResult) (*typed.Relationship, error) {
	value, err := tables.RelationshipTable().Get(txn, key)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return nil, errors.Wrap(err, "could not get record")
		}
		createTimeProto, err := typed.StringToProtobufTimestamp(metadata.CreationTimestamp)
		if err != nil {
			return nil, errors.Wrapf(err, "could not convert string %v to timestamp", metadata.CreationTimestamp)
		}
		value = &typed.Relationship{
			FirstSeen:    rec.Timestamp,
			CreateTime:   createTimeProto,
			DeletedAtEnd: false}
	} else if rec.WatchType == typed.KubeWatchResult_ADD {
		value.FirstSeen = rec.Timestamp
	}
	value.LastSeen = rec.Timestamp
	if rec.WatchType == typed.KubeWatchResult_DELETE {
		value.DeletedAtEnd = true
	}
	return value, nil
}

type InternalResource struct {
	Metadata InternalMeta `json:"metadata"`
	Spec     InternalSpec `json:"spec"`
}

type InternalMeta struct {
	PodName           string       `json:"name"`
	CreationTimestamp string       `json:"creationTimestamp"`
	Spec              InternalSpec `json:"spec"`
	namespace         string       `json:"namespace"`
	uid               string       `json:"uid"`
}

type InternalSpec struct {
	Nodename string `json:"nodeName"`
}

type RelationShipPayload struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"` // pod name
	NameSpace   string `json:"namespace"`
	uid         string `json:"uid"`
	partitionId string `json:"partitionId"`
}
