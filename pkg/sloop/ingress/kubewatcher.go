/*
 * Copyright (c) 2021, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diegoholiveira/jsonlogic/v3"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

/*
This class watches for changes to many kinds of kubernetes resources and writes them to a supplied channel
*/

type KubeWatcher interface {
	Stop()
}

type crdGroupVersionResourceKind struct {
	group    string
	version  string
	resource string
	kind     string
}

type crdInformerInfo struct {
	crd      crdGroupVersionResourceKind
	stopChan chan struct{}
}

type kubeWatcherImpl struct {
	informerFactory informers.SharedInformerFactory
	stopChan        chan struct{}

	crdInformers      map[crdGroupVersionResourceKind]*crdInformerInfo
	activeCrdInformer int64

	outchan        chan typed.KubeWatchResult
	resync         time.Duration
	protection     *sync.Mutex
	stopped        bool
	refreshCrd     *time.Ticker
	currentContext string
	exclusionRules map[string][]any
}

var (
	newCrdClient                        = func(kubeCfg *rest.Config) (clientset.Interface, error) { return clientset.NewForConfig(kubeCfg) }
	metricIngressGranularKubewatchcount = promauto.NewCounterVec(prometheus.CounterOpts{Name: "metric_ingress_event_kubewatchcount"}, []string{"namespace", "name", "kind", "reason", "type"})
	metricIngressKubewatchcount         = promauto.NewCounterVec(prometheus.CounterOpts{Name: "sloop_ingress_kubewatchcount"}, []string{"kind", "watchtype"})
	metricIngressKubewatchbytes         = promauto.NewCounterVec(prometheus.CounterOpts{Name: "sloop_ingress_kubewatchbytes"}, []string{"kind", "watchtype"})
	metricCrdInformerStarted            = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_crd_informer_started"})
	metricCrdInformerRunning            = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_crd_informer_running"})
	metricCrdInformerWatchErrors        = promauto.NewCounterVec(prometheus.CounterOpts{Name: "sloop_crd_informer_watch_errors"}, []string{"kind", "group", "version"})
)

// Todo: Add additional parameters for filtering
func NewKubeWatcherSource(kubeClient kubernetes.Interface, outChan chan typed.KubeWatchResult, resync time.Duration, includeCrds bool, crdRefreshInterval time.Duration, masterURL string, kubeContext string, enableGranularMetrics bool, exclusionRules map[string][]any) (KubeWatcher, error) {
	kw := &kubeWatcherImpl{resync: resync, protection: &sync.Mutex{}}
	kw.stopChan = make(chan struct{})
	kw.crdInformers = make(map[crdGroupVersionResourceKind]*crdInformerInfo)
	kw.outchan = outChan
	kw.exclusionRules = exclusionRules

	kw.startWellKnownInformers(kubeClient, enableGranularMetrics)
	if includeCrds {
		// Arm the ticker BEFORE the first attempt: startCustomInformers fails
		// wholesale on a transient CRD List error, and bailing out here meant
		// the ticker was never created, so CRD watching stayed off for the
		// life of the process while the well-known informers kept running and
		// /healthz stayed green. With the ticker armed first, the refresh
		// loop retries discovery every crdRefreshInterval until it succeeds.
		// The goroutine is started only after the first attempt returns, so
		// two startCustomInformers calls never race over the informer map.
		kw.refreshCrd = time.NewTicker(crdRefreshInterval)
		if err := kw.startCustomInformers(masterURL, kubeContext, enableGranularMetrics); err != nil {
			glog.Errorf("Initial CRD discovery failed, retrying every %v: %v", crdRefreshInterval, err)
		}
		go kw.refreshCrdInformers(masterURL, kubeContext, enableGranularMetrics)
	}

	return kw, nil
}

// stripManagedFields drops metadata.managedFields before an object enters the
// informer cache. Every informer keeps a decoded copy of every object it
// watches, and sloop only ever consumes the event stream - no Lister or Store
// read exists - so the cache is pure overhead that still has to be paid for.
// managedFields is a large, purely administrative part of that: on a GDC
// management plane it was 11% of recorded payload bytes, and it is dead weight
// in the snapshots too. Removing it shrinks the caches, the badger store and
// the backups at once.
//
// The informer owns these objects, so mutating in place is safe and avoids the
// deep copy a non-mutating transform would need.
func stripManagedFields(obj any) (any, error) {
	// A relist after a watch gap delivers deletions as tombstones; strip the
	// object inside rather than letting it through untouched.
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		inner, err := stripManagedFields(tombstone.Obj)
		if err != nil {
			return obj, nil
		}
		tombstone.Obj = inner
		return tombstone, nil
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		// Not a Kubernetes object; pass it through rather than dropping it.
		return obj, nil
	}
	accessor.SetManagedFields(nil)
	return obj, nil
}

// watchInformer wires one informer up to sloop: strip managedFields on the way
// in, then emit each event. SetTransform must be installed before the informer
// starts, so it goes first.
func (i *kubeWatcherImpl) watchInformer(informer cache.SharedIndexInformer, kind string, enableGranularMetrics bool) {
	if err := informer.SetTransform(stripManagedFields); err != nil {
		glog.Errorf("Failed to install managedFields transform for %s: %v", kind, err)
	}
	informer.AddEventHandler(i.getEventHandlerForResource(kind, enableGranularMetrics))
}

func (i *kubeWatcherImpl) startWellKnownInformers(kubeclient kubernetes.Interface, enableGranularMetrics bool) {
	i.informerFactory = informers.NewSharedInformerFactory(kubeclient, i.resync)

	i.watchInformer(i.informerFactory.Apps().V1().DaemonSets().Informer(), "DaemonSet", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Apps().V1().Deployments().Informer(), "Deployment", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Apps().V1().ReplicaSets().Informer(), "ReplicaSet", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Apps().V1().StatefulSets().Informer(), "StatefulSet", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().ConfigMaps().Informer(), "ConfigMap", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().Endpoints().Informer(), "Endpoint", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().Events().Informer(), "Event", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Autoscaling().V1().HorizontalPodAutoscalers().Informer(), "HorizontalPodAutoscaler", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Batch().V1().Jobs().Informer(), "Job", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().Namespaces().Informer(), "Namespace", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().Nodes().Informer(), "Node", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().PersistentVolumeClaims().Informer(), "PersistentVolumeClaim", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().PersistentVolumes().Informer(), "PersistentVolume", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().Pods().Informer(), "Pod", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Policy().V1().PodDisruptionBudgets().Informer(), "PodDisruptionBudget", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().Services().Informer(), "Service", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Core().V1().ReplicationControllers().Informer(), "ReplicationController", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Storage().V1().StorageClasses().Informer(), "StorageClass", enableGranularMetrics)
	i.watchInformer(i.informerFactory.Admissionregistration().V1().MutatingWebhookConfigurations().Informer(), "MutatingWebhookConfiguration", enableGranularMetrics)
	i.informerFactory.Start(i.stopChan)
}

func (i *kubeWatcherImpl) startCustomInformers(masterURL string, kubeContext string, enableGranularMetrics bool) error {
	clientCfg := getConfig(masterURL, kubeContext)
	kubeCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to read config while starting custom informers")
	}

	crdClient, err := newCrdClient(kubeCfg)
	if err != nil {
		return errors.Wrap(err, "failed to instantiate client for querying CRDs")
	}
	crdList, err := getCrdList(crdClient)
	if err != nil {
		return errors.Wrap(err, "failed to query list of CRDs")
	}

	glog.Infof("Found %d CRD definitions", len(crdList))
	dynamicClient, err := dynamic.NewForConfig(kubeCfg)
	if err != nil {
		return errors.Wrap(err, "failed to instantiate client for custom informers")
	}
	existing := i.pullCrdInformers()
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, i.resync, "", nil)
	for _, crd := range crdList {
		i.existingOrStartNewCrdInformer(crd, existing, factory, enableGranularMetrics)
	}

	glog.Infof("Stopping %d CRD Informers", len(existing))
	stopUnwantedCrdInformers(existing)
	metricCrdInformerStarted.Set(float64(len(i.crdInformers)))
	return nil
}

func (i *kubeWatcherImpl) pullCrdInformers() map[crdGroupVersionResourceKind]*crdInformerInfo {
	i.protection.Lock()
	defer i.protection.Unlock()

	crdInformers := i.crdInformers
	i.crdInformers = make(map[crdGroupVersionResourceKind]*crdInformerInfo)
	return crdInformers
}

func (i *kubeWatcherImpl) existingOrStartNewCrdInformer(crd crdGroupVersionResourceKind, existing map[crdGroupVersionResourceKind]*crdInformerInfo, factory dynamicinformer.DynamicSharedInformerFactory, enableGranularMetrics bool) {
	i.protection.Lock()
	defer i.protection.Unlock()
	if i.stopped {
		return
	}
	// if there is an existing informer for this crd, then keep using the existing informer
	crdInformer, found := existing[crd]
	if found {
		i.crdInformers[crd] = crdInformer
		delete(existing, crd) // remove from existing so it wont get stopped as unwanted
		return
	}

	// need an informer for this crd
	crdInformer = &crdInformerInfo{crd: crd, stopChan: make(chan struct{})}
	i.crdInformers[crd] = crdInformer
	i.startNewCrdInformer(crdInformer, factory, enableGranularMetrics)
}

func (i *kubeWatcherImpl) startNewCrdInformer(crdInformer *crdInformerInfo, factory dynamicinformer.DynamicSharedInformerFactory, enableGranularMetrics bool) {
	gvr := schema.GroupVersionResource{Group: crdInformer.crd.group, Version: crdInformer.crd.version, Resource: crdInformer.crd.resource}
	kind := crdInformer.crd.kind
	informer := factory.ForResource(gvr)
	i.watchInformer(informer.Informer(), kind, enableGranularMetrics)
	// Reflector list/watch failures are otherwise invisible: Run() never
	// returns an error and retries forever, and metricCrdInformerRunning is
	// incremented before Run(), so a permanently failing informer (RBAC
	// denied, unreachable apiserver) looks healthy from the outside while
	// recording nothing. Count and log the failures so an empty database can
	// be traced to its cause.
	if err := informer.Informer().SetWatchErrorHandler(func(_ *cache.Reflector, err error) {
		metricCrdInformerWatchErrors.WithLabelValues(kind, gvr.Group, gvr.Version).Inc()
		glog.Errorf("CRD informer list/watch failed for %s (%v): %v", kind, gvr, err)
	}); err != nil {
		glog.Errorf("Failed to install watch error handler for %s (%v): %v", kind, gvr, err)
	}

	go func() {
		glog.V(2).Infof("Starting CRD informer for: %s (%v)", kind, gvr)
		metricCrdInformerRunning.Set(float64(atomic.AddInt64(&i.activeCrdInformer, 1)))

		informer.Informer().Run(crdInformer.stopChan)

		glog.V(2).Infof("Exited CRD informer for: %s (%v)", kind, gvr)
		metricCrdInformerRunning.Set(float64(atomic.AddInt64(&i.activeCrdInformer, -1)))
	}()
}

func stopUnwantedCrdInformers(existing map[crdGroupVersionResourceKind]*crdInformerInfo) {
	// no lock is needed - all these informers should be disconnected from kubeWatcherImpl
	for _, v := range existing {
		gvr := schema.GroupVersionResource{Group: v.crd.group, Version: v.crd.version, Resource: v.crd.resource}
		glog.V(2).Infof("Stopping CRD informer for: %s (%v)", v.crd.kind, gvr)
		close(v.stopChan)
	}
}

func getCrdList(crdClient clientset.Interface) ([]crdGroupVersionResourceKind, error) {
	crdList, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		glog.Errorf("Failed to get CRD list from ApiextensionsV1, falling back to ApiextensionsV1beta1: %v", err)
		return getCrdListV1beta1(crdClient)
	}

	var resources []crdGroupVersionResourceKind
	for _, crd := range crdList.Items {
		// Watch exactly one served version per CRD. The apiserver returns the
		// same objects through every served version, so watching all of them
		// wrote each object N times (observed: 220 watch events for 110
		// Projects on a v1+v1alpha1 CRD), and an unserved version has no
		// endpoint at all - its informer 404-loops forever while being
		// counted as running. Prefer the storage version when it is served,
		// else fall back to the first served one.
		chosen := ""
		for _, version := range crd.Spec.Versions {
			if !version.Served {
				continue
			}
			if chosen == "" {
				chosen = version.Name
			}
			if version.Storage {
				chosen = version.Name
				break
			}
		}
		if chosen == "" {
			glog.V(2).Infof("CRD %s kind %s has no served versions; skipping", crd.Spec.Group, crd.Spec.Names.Kind)
			continue
		}
		gvrk := crdGroupVersionResourceKind{group: crd.Spec.Group, version: chosen, resource: crd.Spec.Names.Plural, kind: crd.Spec.Names.Kind}
		glog.V(2).Infof("CRD: group: %s, version: %s, kind: %s, plural:%s, singular:%s, short names:%v", crd.Spec.Group, chosen, crd.Spec.Names.Kind, crd.Spec.Names.Plural, crd.Spec.Names.Singular, crd.Spec.Names.ShortNames)
		resources = append(resources, gvrk)
	}
	return resources, nil
}

func getCrdListV1beta1(crdClient clientset.Interface) ([]crdGroupVersionResourceKind, error) {
	crdList, err := crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to query CRDs")
	}

	// duplicated code (see getCrdList), the types for crdList are different
	var resources []crdGroupVersionResourceKind
	for _, crd := range crdList.Items {
		// See getCrdList: watch exactly one served version per CRD,
		// preferring the storage version.
		chosen := ""
		for _, version := range crd.Spec.Versions {
			if !version.Served {
				continue
			}
			if chosen == "" {
				chosen = version.Name
			}
			if version.Storage {
				chosen = version.Name
				break
			}
		}
		if chosen == "" {
			glog.V(2).Infof("CRD %s kind %s has no served versions; skipping", crd.Spec.Group, crd.Spec.Names.Kind)
			continue
		}
		gvrk := crdGroupVersionResourceKind{group: crd.Spec.Group, version: chosen, resource: crd.Spec.Names.Plural, kind: crd.Spec.Names.Kind}
		glog.V(2).Infof("CRD: group: %s, version: %s, kind: %s, plural:%s, singular:%s, short names:%v", crd.Spec.Group, chosen, crd.Spec.Names.Kind, crd.Spec.Names.Plural, crd.Spec.Names.Singular, crd.Spec.Names.ShortNames)
		resources = append(resources, gvrk)
	}
	return resources, nil
}

func (i *kubeWatcherImpl) getEventHandlerForResource(resourceKind string, enableGranularMetrics bool) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    i.reportAdd(resourceKind, enableGranularMetrics),
		DeleteFunc: i.reportDelete(resourceKind, enableGranularMetrics),
		UpdateFunc: i.reportUpdate(resourceKind, enableGranularMetrics),
	}
}

func (i *kubeWatcherImpl) reportAdd(kind string, enableGranularMetrics bool) func(interface{}) {
	return func(obj interface{}) {
		watchResultShell := &typed.KubeWatchResult{
			Timestamp: ptypes.TimestampNow(),
			Kind:      kind,
			WatchType: typed.KubeWatchResult_ADD,
			Payload:   "",
		}
		i.processUpdate(kind, obj, watchResultShell, enableGranularMetrics)
	}
}

func (i *kubeWatcherImpl) reportDelete(kind string, enableGranularMetrics bool) func(interface{}) {
	return func(obj interface{}) {
		delObj, ok := obj.(cache.DeletedFinalStateUnknown)
		if ok {
			obj = delObj.Obj
		}

		watchResultShell := &typed.KubeWatchResult{
			Timestamp: ptypes.TimestampNow(),
			Kind:      kind,
			WatchType: typed.KubeWatchResult_DELETE,
			Payload:   "",
		}
		i.processUpdate(kind, obj, watchResultShell, enableGranularMetrics)
	}
}

func (i *kubeWatcherImpl) reportUpdate(kind string, enableGranularMetrics bool) func(interface{}, interface{}) {
	return func(_ interface{}, newObj interface{}) {
		watchResultShell := &typed.KubeWatchResult{
			Timestamp: ptypes.TimestampNow(),
			Kind:      kind,
			WatchType: typed.KubeWatchResult_UPDATE,
			Payload:   "",
		}
		i.processUpdate(kind, newObj, watchResultShell, enableGranularMetrics)
	}
}

func (i *kubeWatcherImpl) processUpdate(kind string, obj interface{}, watchResult *typed.KubeWatchResult, enableGranularmetrics bool) {
	resourceJson, err := i.getResourceAsJsonString(kind, obj)
	if err != nil {
		glog.Error(err)
		return
	}
	glog.V(99).Infof("processUpdate: obj json: %v", resourceJson)

	eventExcluded := i.eventExcluded(kind, resourceJson)
	if eventExcluded {
		objName := reflect.ValueOf(obj).Elem().FieldByName("ObjectMeta").FieldByName("Name")
		glog.V(2).Infof("Event for object excluded: %s/%s", kind, objName)
		return
	}

	kubeMetadata, err := kubeextractor.ExtractMetadata(resourceJson)
	if err != nil || kubeMetadata.Namespace == "" {
		// We are only grabbing namespace here for a prometheus metric, so if metadata extract fails we just log and continue
		glog.V(2).Infof("No namespace for resource: %v", err)
	}
	if enableGranularmetrics && kind == "Event" {
		eventInfo, err1 := kubeextractor.ExtractEventInfo(resourceJson)
		involvedObject, err2 := kubeextractor.ExtractInvolvedObject(resourceJson)
		if err1 != nil {
			glog.V(2).Infof("Extract event info: %v", err1)
		}
		if err2 != nil {
			glog.V(2).Infof("Error occurred while extracting Involved Object Info: %v", err2)
		}
		metricIngressGranularKubewatchcount.WithLabelValues(involvedObject.Namespace, involvedObject.Name, involvedObject.Kind, eventInfo.Reason, eventInfo.Type).Inc()
		glog.V(common.GlogVerbose).Infof("Informer update: Name: %s, Namespace: %s, Reason: %s, Type: %s", involvedObject.Name, involvedObject.Namespace, eventInfo.Reason, eventInfo.Type)
	}

	metricIngressKubewatchcount.WithLabelValues(kind, watchResult.WatchType.String()).Inc()
	metricIngressKubewatchbytes.WithLabelValues(kind, watchResult.WatchType.String()).Add(float64(len(resourceJson)))

	glog.V(common.GlogVerbose).Infof("Informer update (%s) - Name: %s, Namespace: %s, ResourceVersion: %s", watchResult.WatchType, kubeMetadata.Name, kubeMetadata.Namespace, kubeMetadata.ResourceVersion)
	watchResult.Payload = resourceJson
	i.writeToOutChan(watchResult)
}

func (i *kubeWatcherImpl) writeToOutChan(watchResult *typed.KubeWatchResult) {
	// We need to ensure that no messages are written to outChan after stop is called.
	// The lock only guards the stopped check; we must NOT hold it across the channel
	// send, otherwise a full channel would block here while holding i.protection, which
	// deadlocks any other path that needs the lock (e.g. starting CRD informers during
	// the initial sync of a cluster with many CRDs).
	i.protection.Lock()
	stopped := i.stopped
	i.protection.Unlock()
	if stopped {
		return
	}

	// Send without holding the lock. Select on stopChan so that the send unblocks if the
	// watcher is stopped while the channel is full, instead of blocking forever.
	select {
	case i.outchan <- *watchResult:
	case <-i.stopChan:
	}
}

func (i *kubeWatcherImpl) getResourceAsJsonString(kind string, obj interface{}) (string, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("resource cannot be marshalled %v", err)
	}

	return string(bytes), nil
}

func (i *kubeWatcherImpl) refreshCrdInformers(masterURL string, kubeContext string, enableGranularMetrics bool) {
	for range i.refreshCrd.C {
		glog.V(common.GlogVerbose).Infof("Starting to refresh CRD informers")
		err := i.startCustomInformers(masterURL, kubeContext, enableGranularMetrics)
		if err != nil {
			glog.Errorf("Failed to refresh CRD informers: %v", err)
		}
	}
}

func (i *kubeWatcherImpl) getExclusionRules(kind string) []any {
	kindRules, _ := i.exclusionRules[kind]
	globalRules, _ := i.exclusionRules["_all"]
	combinedRules := append(
		kindRules,
		globalRules...,
	)
	glog.V(common.GlogVerbose).Infof("Fetched rules: %s", combinedRules)
	return combinedRules
}

func (i *kubeWatcherImpl) eventExcluded(kind string, resourceJson string) bool {
	filters := i.getExclusionRules(kind)
	for _, logic := range filters {
		logicJson, err := json.Marshal(logic)
		if err != nil {
			glog.Errorf(`Failed to parse event filtering rule "%s": %s`, string(logicJson), err)
			return false
		}
		var result bytes.Buffer
		err = jsonlogic.Apply(
			strings.NewReader(string(logicJson)),
			strings.NewReader(resourceJson),
			&result,
		)
		if err != nil {
			glog.Errorf(`Failed to apply event filtering rule "%s": %s`, string(logicJson), err)
			return false
		}
		resultBool := strings.Contains(result.String(), "true")
		if resultBool {
			truncated, _ := common.Truncate(resourceJson, 40)
			glog.V(2).Infof(`Event matched logic: logic="%s" resource="%s"`, string(logicJson), truncated)
			return true
		}
	}
	return false
}

func (i *kubeWatcherImpl) Stop() {
	glog.Infof("Stopping kubeWatcher")

	i.protection.Lock()
	if i.stopped {
		return
	}
	i.stopped = true
	i.protection.Unlock()

	if i.refreshCrd != nil {
		i.refreshCrd.Stop()
	}

	close(i.stopChan)
	stopUnwantedCrdInformers(i.pullCrdInformers())
}
