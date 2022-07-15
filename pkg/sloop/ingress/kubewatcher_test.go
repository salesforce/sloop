/*
 * Copyright (c) 2021, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package ingress

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	clientsetFake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	kubernetesFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8sTesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

type dummyData struct {
	Namespace string
}

// when fake client tries to list CRDs, return a list with one defined
func reactionListOfOne(_ k8sTesting.Action) (bool, runtime.Object, error) {
	versions := []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1"}}
	name := apiextensionsv1.CustomResourceDefinitionNames{Plural: "things", Kind: "k"}
	spec := apiextensionsv1.CustomResourceDefinitionSpec{Group: "g", Versions: versions, Names: name}
	crd := apiextensionsv1.CustomResourceDefinition{Spec: spec}
	list := apiextensionsv1.CustomResourceDefinitionList{Items: []apiextensionsv1.CustomResourceDefinition{crd}}
	return true, &list, nil
}

// when fake client tries to list CRDs, return an error
func reactionError(_ k8sTesting.Action) (bool, runtime.Object, error) {
	return true, nil, fmt.Errorf("failed")
}

// newTestCrdClient - provides a function pointer to create a fake clientset
//   takes: k8sTesting.Action function pointer - adds the reaction to the fake clientset
//   returns a function pointer
//      takes: restConfig
//      returns: clientset.Interface & error (always nil)
func newTestCrdClient(reaction func(_ k8sTesting.Action) (bool, runtime.Object, error)) func(_ *rest.Config) (clientset.Interface, error) {
	return func(_ *rest.Config) (clientset.Interface, error) {
		crdClient := &clientsetFake.Clientset{}
		crdClient.AddReactor("list", "*", reaction)
		return crdClient, nil
	}
}

// This test (test-harness) exercises the kubewatcher from the client perspective
// - start a kubewatcher
// - force a k8s event in the system
// - wait for an event
// - cleanup
func Test_bigPicture(t *testing.T) {
	newCrdClient = newTestCrdClient(reactionListOfOne) // force startCustomInformers() to use a fake clientset

	kubeClient := kubernetesFake.NewSimpleClientset()
	outChan := make(chan typed.KubeWatchResult, 5)
	resync := 30 * time.Minute
	includeCrds := true
	masterURL := "url"
	kubeContext := "" // empty string makes things work
	enableGranularMetrics := true
	kw, err := NewKubeWatcherSource(kubeClient, outChan, resync, includeCrds, time.Duration(10*time.Second), masterURL, kubeContext, enableGranularMetrics)
	assert.NoError(t, err)

	// create service and await corresponding event
	ns := "ns"
	_, err = kubeClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	if err != nil {
		t.FailNow()
	}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "s"}}
	_, err = kubeClient.CoreV1().Services(ns).Create(svc)
	if err != nil {
		t.Fatalf("Error creating service: %v\n", err)
	}
	_ = <-outChan

	kw.Stop()
}

func Test_getCrdList(t *testing.T) {
	crdClient, _ := newTestCrdClient(reactionError)(&rest.Config{})
	crdList, err := getCrdList(crdClient)
	assert.Error(t, err)

	crdClient, _ = newTestCrdClient(reactionListOfOne)(&rest.Config{})
	crdList, err = getCrdList(crdClient)
	assert.Len(t, crdList, 1)
	assert.NoError(t, err)
}

func Test_getEventHandlerForResource(t *testing.T) {
	kw := &kubeWatcherImpl{protection: &sync.Mutex{}}
	enableGranularMetrics := true
	handler, ok := kw.getEventHandlerForResource("k", enableGranularMetrics).(cache.ResourceEventHandlerFuncs)
	assert.True(t, ok)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.AddFunc)
	assert.NotNil(t, handler.DeleteFunc)
	assert.NotNil(t, handler.UpdateFunc)
}

func Test_reportAdd(t *testing.T) {
	outChan := make(chan typed.KubeWatchResult, 5)
	kw := &kubeWatcherImpl{protection: &sync.Mutex{}, outchan: outChan}
	enableGranularMetrics := true
	kind := "a"
	report := kw.reportAdd(kind, enableGranularMetrics)
	assert.NotNil(t, report)
	obj := dummyData{Namespace: "n"}
	bytes, err := json.Marshal(obj)
	assert.Nil(t, err)

	report(obj)

	result := <-outChan
	assert.Equal(t, kind, result.Kind)
	assert.Equal(t, typed.KubeWatchResult_ADD, result.WatchType)
	assert.Equal(t, string(bytes), result.Payload)

	verifyChannelEmpty(t, outChan)
}

func Test_reportDelete(t *testing.T) {
	outChan := make(chan typed.KubeWatchResult, 5)
	kw := &kubeWatcherImpl{protection: &sync.Mutex{}, outchan: outChan}

	kind := "d"
	enableGranularMetrics := true
	report := kw.reportDelete(kind, enableGranularMetrics)
	assert.NotNil(t, report)
	obj := dummyData{Namespace: "n"}
	bytes, err := json.Marshal(obj)
	assert.Nil(t, err)

	report(obj)

	result := <-outChan
	assert.Equal(t, kind, result.Kind)
	assert.Equal(t, typed.KubeWatchResult_DELETE, result.WatchType)
	assert.Equal(t, string(bytes), result.Payload)

	deleteObj := cache.DeletedFinalStateUnknown{
		Key: "object-key",
		Obj: obj,
	}
	report(deleteObj)
	result = <-outChan
	assert.Equal(t, string(bytes), result.Payload)

	verifyChannelEmpty(t, outChan)
}

func Test_reportUpdate(t *testing.T) {
	outChan := make(chan typed.KubeWatchResult, 5)
	kw := &kubeWatcherImpl{protection: &sync.Mutex{}, outchan: outChan}

	kind := "d"
	enableGranularMetrics := true
	report := kw.reportUpdate(kind, enableGranularMetrics)
	assert.NotNil(t, report)
	prev := dummyData{Namespace: "p"}
	new := dummyData{Namespace: "n"}
	bytes, err := json.Marshal(new)
	assert.Nil(t, err)

	report(prev, new)

	result := <-outChan
	assert.Equal(t, kind, result.Kind)
	assert.Equal(t, typed.KubeWatchResult_UPDATE, result.WatchType)
	assert.Equal(t, string(bytes), result.Payload)

	verifyChannelEmpty(t, outChan)
}

func Test_processUpdate(t *testing.T) {
	outChan := make(chan typed.KubeWatchResult, 5)
	kw := &kubeWatcherImpl{protection: &sync.Mutex{}, outchan: outChan}

	kind := "k"
	obj := dummyData{Namespace: "n"}
	enableGranularMetrics := true
	kw.processUpdate(kind, obj, &typed.KubeWatchResult{Kind: kind}, enableGranularMetrics)
	result := <-outChan
	assert.Equal(t, kind, result.Kind)
	assert.NotEmpty(t, result.Payload)

	verifyChannelEmpty(t, outChan)
}

func verifyChannelEmpty(t *testing.T, outChan chan typed.KubeWatchResult) {
	select {
	case _, _ = <-outChan:
		assert.Fail(t, "expected channel to be empty")
	default:
		// channel is empty
	}
}

func Test_existingOrStartNewCrdInformer(t *testing.T) {
	kw := &kubeWatcherImpl{protection: &sync.Mutex{}}
	kw.crdInformers = make(map[crdGroupVersionResourceKind]*crdInformerInfo)

	client := dynamicFake.NewSimpleDynamicClient(runtime.NewScheme())
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, 30*time.Minute, "", nil)

	crd := crdGroupVersionResourceKind{group: "g", version: "v", resource: "r", kind: "k"}
	existing := make(map[crdGroupVersionResourceKind]*crdInformerInfo)
	enableGranularMetrics := true
	// start new informer
	kw.existingOrStartNewCrdInformer(crd, existing, factory, enableGranularMetrics)
	assert.Len(t, kw.crdInformers, 1)

	for atomic.LoadInt64(&kw.activeCrdInformer) == 0 { // wait for the go routine to start
		time.Sleep(time.Millisecond)
	}

	// refresh - start existing informer
	existing = kw.pullCrdInformers()
	assert.Len(t, kw.crdInformers, 0)
	assert.Len(t, existing, 1)

	kw.existingOrStartNewCrdInformer(crd, existing, factory, enableGranularMetrics)
	assert.Len(t, kw.crdInformers, 1)
	assert.Len(t, existing, 0)
	assert.Equal(t, int64(1), atomic.LoadInt64(&kw.activeCrdInformer))

	// cleanup the informer
	stopUnwantedCrdInformers(kw.pullCrdInformers())
	for atomic.LoadInt64(&kw.activeCrdInformer) != 0 { // wait for the go routine to exit
		time.Sleep(time.Millisecond)
	}
}
