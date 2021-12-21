package typed

import (
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)


func Test_RelationshipTableKey_OutputCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	k := NewRelationshipKey(someTs, someKind, someNamespace, someName, someUid)
	assert.Equal(t, "/relation/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8", k.String())
}

func Test_RelationshipTableKey_ParseCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	k := &RelationshipKey{}
	err := k.Parse("/relation/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8")
	assert.Nil(t, err)
	assert.Equal(t, "001546398000", k.PartitionId)
	assert.Equal(t, someNamespace, k.Namespace)
	assert.Equal(t, someName, k.Name)
	assert.Equal(t, someUid, k.Uid)
}



func Test_RelationshipTableKey_ValidateWorks(t *testing.T) {
	testKey := "/relation/001562961600/ReplicaSet/mesh-control-plane/istio-pilot-56f7d9848/1562963507608345756"
	assert.Nil(t, (&RelationshipKey{}).ValidateKey(testKey))
}


