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
		err := kw.startCustomInformers(masterURL, kubeContext, enableGranularMetrics)
		if err != nil {
			return nil, err
		}

		kw.refreshCrd = time.NewTicker(crdRefreshInterval)
		go kw.refreshCrdInformers(masterURL, kubeContext, enableGranularMetrics)
	}

	return kw, nil
}

func (i *kubeWatcherImpl) startWellKnownInformers(kubeclient kubernetes.Interface, enableGranularMetrics bool) {
	i.informerFactory = informers.NewSharedInformerFactory(kubeclient, i.resync)

	i.informerFactory.Apps().V1().DaemonSets().Informer().AddEventHandler(i.getEventHandlerForResource("DaemonSet", enableGranularMetrics))
	i.informerFactory.Apps().V1().Deployments().Informer().AddEventHandler(i.getEventHandlerForResource("Deployment", enableGranularMetrics))
	i.informerFactory.Apps().V1().ReplicaSets().Informer().AddEventHandler(i.getEventHandlerForResource("ReplicaSet", enableGranularMetrics))
	i.informerFactory.Apps().V1().StatefulSets().Informer().AddEventHandler(i.getEventHandlerForResource("StatefulSet", enableGranularMetrics))
	i.informerFactory.Core().V1().ConfigMaps().Informer().AddEventHandler(i.getEventHandlerForResource("ConfigMap", enableGranularMetrics))
	i.informerFactory.Core().V1().Endpoints().Informer().AddEventHandler(i.getEventHandlerForResource("Endpoint", enableGranularMetrics))
	i.informerFactory.Core().V1().Events().Informer().AddEventHandler(i.getEventHandlerForResource("Event", enableGranularMetrics))
	i.informerFactory.Autoscaling().V1().HorizontalPodAutoscalers().Informer().AddEventHandler(i.getEventHandlerForResource("HorizontalPodAutoscaler", enableGranularMetrics))
	i.informerFactory.Batch().V1().Jobs().Informer().AddEventHandler(i.getEventHandlerForResource("Job", enableGranularMetrics))
	i.informerFactory.Core().V1().Namespaces().Informer().AddEventHandler(i.getEventHandlerForResource("Namespace", enableGranularMetrics))
	i.informerFactory.Core().V1().Nodes().Informer().AddEventHandler(i.getEventHandlerForResource("Node", enableGranularMetrics))
	i.informerFactory.Core().V1().PersistentVolumeClaims().Informer().AddEventHandler(i.getEventHandlerForResource("PersistentVolumeClaim", enableGranularMetrics))
	i.informerFactory.Core().V1().PersistentVolumes().Informer().AddEventHandler(i.getEventHandlerForResource("PersistentVolume", enableGranularMetrics))
	i.informerFactory.Core().V1().Pods().Informer().AddEventHandler(i.getEventHandlerForResource("Pod", enableGranularMetrics))
	i.informerFactory.Policy().V1().PodDisruptionBudgets().Informer().AddEventHandler(i.getEventHandlerForResource("PodDisruptionBudget", enableGranularMetrics))
	i.informerFactory.Core().V1().Services().Informer().AddEventHandler(i.getEventHandlerForResource("Service", enableGranularMetrics))
	i.informerFactory.Core().V1().ReplicationControllers().Informer().AddEventHandler(i.getEventHandlerForResource("ReplicationController", enableGranularMetrics))
	i.informerFactory.Storage().V1().StorageClasses().Informer().AddEventHandler(i.getEventHandlerForResource("StorageClass", enableGranularMetrics))
	i.informerFactory.Admissionregistration().V1().MutatingWebhookConfigurations().Informer().AddEventHandler(i.getEventHandlerForResource("MutatingWebhookConfiguration", enableGranularMetrics))
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
	informer.Informer().AddEventHandler(i.getEventHandlerForResource(kind, enableGranularMetrics))

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
		for _, version := range crd.Spec.Versions {
			gvrk := crdGroupVersionResourceKind{group: crd.Spec.Group, version: version.Name, resource: crd.Spec.Names.Plural, kind: crd.Spec.Names.Kind}
			glog.V(2).Infof("CRD: group: %s, version: %s, kind: %s, plural:%s, singular:%s, short names:%v", crd.Spec.Group, version.Name, crd.Spec.Names.Kind, crd.Spec.Names.Plural, crd.Spec.Names.Singular, crd.Spec.Names.ShortNames)
			resources = append(resources, gvrk)
		}
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
		for _, version := range crd.Spec.Versions {
			gvrk := crdGroupVersionResourceKind{group: crd.Spec.Group, version: version.Name, resource: crd.Spec.Names.Plural, kind: crd.Spec.Names.Kind}
			glog.V(2).Infof("CRD: group: %s, version: %s, kind: %s, plural:%s, singular:%s, short names:%v", crd.Spec.Group, version.Name, crd.Spec.Names.Kind, crd.Spec.Names.Plural, crd.Spec.Names.Singular, crd.Spec.Names.ShortNames)
			resources = append(resources, gvrk)
		}
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
	// We need to ensure that no messages are written to outChan after stop is called
	// Kube watch library has a way to tell it to stop, but no way to know it is complete
	// Use a lock around output channel for this purpose
	i.protection.Lock()
	defer i.protection.Unlock()
	if i.stopped {
		return
	}
	i.outchan <- *watchResult // WARNING - if this channel gets full, this push will block while holding i.protection in a locked state
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
