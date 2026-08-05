/*
 * Copyright (c) 2021, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package ingress

import (
	"context"
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
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
	versions := []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Served: true, Storage: true}}
	name := apiextensionsv1.CustomResourceDefinitionNames{Plural: "things", Kind: "k"}
	spec := apiextensionsv1.CustomResourceDefinitionSpec{Group: "g", Versions: versions, Names: name}
	crd := apiextensionsv1.CustomResourceDefinition{Spec: spec}
	list := apiextensionsv1.CustomResourceDefinitionList{Items: []apiextensionsv1.CustomResourceDefinition{crd}}
	return true, &list, nil
}

// when fake client tries to list CRDs, return one CRD that serves two
// versions (storage version listed second) plus one CRD with no served
// version at all: getCrdList must pick exactly the storage version of the
// first and skip the second entirely.
func reactionMultiVersion(_ k8sTesting.Action) (bool, runtime.Object, error) {
	dual := apiextensionsv1.CustomResourceDefinition{Spec: apiextensionsv1.CustomResourceDefinitionSpec{
		Group: "g",
		Names: apiextensionsv1.CustomResourceDefinitionNames{Plural: "things", Kind: "k"},
		Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
			{Name: "v1alpha1", Served: true, Storage: false},
			{Name: "v1", Served: true, Storage: true},
		},
	}}
	unserved := apiextensionsv1.CustomResourceDefinition{Spec: apiextensionsv1.CustomResourceDefinitionSpec{
		Group: "g2",
		Names: apiextensionsv1.CustomResourceDefinitionNames{Plural: "others", Kind: "o"},
		Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
			{Name: "v1", Served: false, Storage: true},
		},
	}}
	list := apiextensionsv1.CustomResourceDefinitionList{Items: []apiextensionsv1.CustomResourceDefinition{dual, unserved}}
	return true, &list, nil
}

// when fake client tries to list CRDs, return an error
func reactionError(_ k8sTesting.Action) (bool, runtime.Object, error) {
	return true, nil, fmt.Errorf("failed")
}

// newTestCrdClient - provides a function pointer to create a fake clientset
//
//	takes: k8sTesting.Action function pointer - adds the reaction to the fake clientset
//	returns a function pointer
//	   takes: restConfig
//	   returns: clientset.Interface & error (always nil)
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
	exclusionRules := map[string][]any{}
	kw, err := NewKubeWatcherSource(kubeClient, outChan, resync, includeCrds, time.Duration(10*time.Second), masterURL, kubeContext, enableGranularMetrics, exclusionRules)
	assert.NoError(t, err)

	// create service and await corresponding event
	ns := "ns"
	_, err = kubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "s"}}
	_, err = kubeClient.CoreV1().Services(ns).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating service: %v\n", err)
	}
	_ = <-outChan

	kw.Stop()
}

// As above but specify non-default exclusion rules to exclude events for service named s2
func Test_bigPictureWithExclusionRules(t *testing.T) {
	newCrdClient = newTestCrdClient(reactionListOfOne) // force startCustomInformers() to use a fake clientset

	kubeClient := kubernetesFake.NewSimpleClientset()
	outChan := make(chan typed.KubeWatchResult, 5)
	resync := 30 * time.Minute
	includeCrds := true
	masterURL := "url"
	kubeContext := "" // empty string makes things work
	enableGranularMetrics := true
	exclusionRules := map[string][]any{
		"_all": []any{
			map[string]any{
				"==": []any{
					map[string]any{
						"var": "metadata.name",
					},
					"s2",
				},
			},
		},
	}

	kw, err := NewKubeWatcherSource(kubeClient, outChan, resync, includeCrds, time.Duration(10*time.Second), masterURL, kubeContext, enableGranularMetrics, exclusionRules)
	assert.NoError(t, err)

	// create namespace
	ns := "ns"
	_, err = kubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}, metav1.CreateOptions{})
	if err != nil {
		t.FailNow()
	}

	// create first service
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "s1"}}
	_, err = kubeClient.CoreV1().Services(ns).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating service: %v\n", err)
	}

	// create second service, corresponding event should be excluded by exclusion rule
	svc = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "s2"}}
	_, err = kubeClient.CoreV1().Services(ns).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating service: %v\n", err)
	}

	// create third service
	svc = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "s3"}}
	_, err = kubeClient.CoreV1().Services(ns).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating service: %v\n", err)
	}

	eventCount := 0
loop:
	for {
		select {
		case <-time.After(1 * time.Second):
			break loop
		case result, ok := <-outChan:
			if ok {
				eventCount++
				assert.NotContains(t, result.Payload, `"name":"s2"`)
			} else {
				t.Fatalf("Channel closed unexpectedly: %v\n", ok)
			}
		}
	}
	assert.Equal(t, 3, eventCount) // assert no event for service named s2

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

	// One informer per CRD, on the served storage version; CRDs with no
	// served version are skipped entirely.
	crdClient, _ = newTestCrdClient(reactionMultiVersion)(&rest.Config{})
	crdList, err = getCrdList(crdClient)
	assert.NoError(t, err)
	assert.Len(t, crdList, 1)
	assert.Equal(t, "v1", crdList[0].version)
	assert.Equal(t, "k", crdList[0].kind)
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

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "g", Version: "v", Resource: "r"}: "kList",
	}

	client := dynamicFake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind)
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

func Test_stripManagedFields(t *testing.T) {
	managed := []metav1.ManagedFieldsEntry{{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply}}

	// Typed object.
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", ManagedFields: managed}}
	out, err := stripManagedFields(pod)
	assert.Nil(t, err)
	assert.Empty(t, out.(*corev1.Pod).ManagedFields)
	assert.Equal(t, "p1", out.(*corev1.Pod).Name)

	// Unstructured object, as delivered by the CRD (dynamic) informers.
	custom := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "resourcemanager.gdc.goog/v1alpha1",
		"kind":       "Project",
		"metadata": map[string]any{
			"name":          "proj1",
			"managedFields": []any{map[string]any{"manager": "kubectl"}},
		},
	}}
	out, err = stripManagedFields(custom)
	assert.Nil(t, err)
	md := out.(*unstructured.Unstructured).Object["metadata"].(map[string]any)
	_, stillThere := md["managedFields"]
	assert.False(t, stillThere)
	assert.Equal(t, "proj1", md["name"])

	// A tombstone wraps the pointer the store already holds, which the handler
	// goroutine may be serializing; it must be handed back untouched rather
	// than written to a second time.
	tombPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", ManagedFields: managed}}
	out, err = stripManagedFields(cache.DeletedFinalStateUnknown{Key: "ns/p2", Obj: tombPod})
	assert.Nil(t, err)
	assert.Equal(t, managed, out.(cache.DeletedFinalStateUnknown).Obj.(*corev1.Pod).ManagedFields)

	// Second pass over an already-stripped object must not write again.
	stripped := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p3", ManagedFields: managed}}
	_, err = stripManagedFields(stripped)
	assert.Nil(t, err)
	assert.Empty(t, stripped.ManagedFields)
	before := stripped.ManagedFields
	_, err = stripManagedFields(stripped)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%p", before), fmt.Sprintf("%p", stripped.ManagedFields))

	// Non-Kubernetes payload passes through instead of being dropped.
	out, err = stripManagedFields("not-an-object")
	assert.Nil(t, err)
	assert.Equal(t, "not-an-object", out)
}

// The DeltaFIFO applies the transform to every delta, not once per object: a
// resync re-runs it against the pointer already in the store, which is the same
// one the event handler is holding. This test fails under -race if the
// transform writes to the object on that second pass. The informer floor for
// resyncPeriod is 1s, so the window has to outlast it.
func Test_stripManagedFieldsNoWriteOnResync(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "resourcemanager.gdc.goog/v1alpha1",
		"kind":       "Project",
		"metadata": map[string]any{
			"name":          "p1",
			"namespace":     "ns",
			"managedFields": []any{map[string]any{"manager": "kubectl"}},
		},
	}}
	lw := &cache.ListWatch{
		ListFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
			return &unstructured.UnstructuredList{
				Object: map[string]any{"apiVersion": "v1", "kind": "List", "metadata": map[string]any{"resourceVersion": "1"}},
				Items:  []unstructured.Unstructured{*obj},
			}, nil
		},
		WatchFunc: func(_ metav1.ListOptions) (watch.Interface, error) { return watch.NewFake(), nil },
	}

	informer := cache.NewSharedIndexInformer(lw, &unstructured.Unstructured{}, time.Second, cache.Indexers{})
	assert.Nil(t, informer.SetTransform(stripManagedFields))

	delivered := make(chan any, 1)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(o any) {
			select {
			case delivered <- o:
			default:
			}
		},
	})

	stop := make(chan struct{})
	var running sync.WaitGroup
	running.Add(1)
	go func() {
		defer running.Done()
		informer.Run(stop)
	}()
	assert.True(t, cache.WaitForCacheSync(stop, informer.HasSynced))

	got := <-delivered
	// Serialize the delivered object the way getResourceAsJsonString does,
	// spanning two resync ticks.
	deadline := time.Now().Add(2500 * time.Millisecond)
	var marshaling sync.WaitGroup
	marshaling.Add(1)
	go func() {
		defer marshaling.Done()
		for time.Now().Before(deadline) {
			_, _ = json.Marshal(got)
		}
	}()
	marshaling.Wait()

	// Shut the informer down before returning so no goroutine outlives the test.
	close(stop)
	running.Wait()

	md := got.(*unstructured.Unstructured).Object["metadata"].(map[string]any)
	_, stillThere := md["managedFields"]
	assert.False(t, stillThere)
}
