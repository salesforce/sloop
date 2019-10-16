/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package ingress

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
)

/*
This class watches for changes to many kinds of kubernetes resources and writes them to a supplied channel
*/

type KubeWatcher interface {
	Stop()
}

type kubeWatcherImpl struct {
	informerFactory informers.SharedInformerFactory
	stopChan        chan struct{}
	outchan         chan typed.KubeWatchResult
	resync          time.Duration
	outchanlock     *sync.Mutex
	stopped         bool
	currentContext  string
}

var (
	metricIngressKubewatchcount = promauto.NewCounterVec(prometheus.CounterOpts{Name: "sloop_ingress_kubewatchcount"}, []string{"kind", "watchtype", "namespace"})
	metricIngressKubewatchbytes = promauto.NewCounterVec(prometheus.CounterOpts{Name: "sloop_ingress_kubewatchbytes"}, []string{"kind", "watchtype", "namespace"})
)

// Todo: Add additional parameters for filtering
func NewKubeWatcherSource(kubeClient kubernetes.Interface, outChan chan typed.KubeWatchResult, resync time.Duration, includeCrds bool, masterURL string, kubeContext string) (KubeWatcher, error) {
	kw := &kubeWatcherImpl{resync: resync, outchanlock: &sync.Mutex{}}
	kw.stopChan = make(chan struct{})
	kw.outchan = outChan

	kw.startWellKnownInformers(kubeClient)
	if includeCrds {
		err := kw.startCustomInformers(masterURL, kubeContext)
		if err != nil {
			return nil, err
		}
	}

	return kw, nil
}

func (i *kubeWatcherImpl) startWellKnownInformers(kubeclient kubernetes.Interface) {
	i.informerFactory = informers.NewSharedInformerFactory(kubeclient, i.resync)

	i.informerFactory.Apps().V1beta1().Deployments().Informer().AddEventHandler(i.getEventHandlerForResource("Deployment"))
	i.informerFactory.Apps().V1beta1().StatefulSets().Informer().AddEventHandler(i.getEventHandlerForResource("StatefulSet"))
	i.informerFactory.Core().V1().ConfigMaps().Informer().AddEventHandler(i.getEventHandlerForResource("ConfigMap"))
	i.informerFactory.Core().V1().Endpoints().Informer().AddEventHandler(i.getEventHandlerForResource("Endpoint"))
	i.informerFactory.Core().V1().Namespaces().Informer().AddEventHandler(i.getEventHandlerForResource("Namespace"))
	i.informerFactory.Core().V1().Nodes().Informer().AddEventHandler(i.getEventHandlerForResource("Node"))
	i.informerFactory.Core().V1().PersistentVolumeClaims().Informer().AddEventHandler(i.getEventHandlerForResource("PersistentVolumeClaim"))
	i.informerFactory.Core().V1().PersistentVolumes().Informer().AddEventHandler(i.getEventHandlerForResource("PersistentVolume"))
	i.informerFactory.Core().V1().Pods().Informer().AddEventHandler(i.getEventHandlerForResource("Pod"))
	i.informerFactory.Core().V1().Services().Informer().AddEventHandler(i.getEventHandlerForResource("Service"))
	i.informerFactory.Core().V1().ReplicationControllers().Informer().AddEventHandler(i.getEventHandlerForResource("ReplicationController"))
	i.informerFactory.Extensions().V1beta1().DaemonSets().Informer().AddEventHandler(i.getEventHandlerForResource("DaemonSet"))
	i.informerFactory.Extensions().V1beta1().ReplicaSets().Informer().AddEventHandler(i.getEventHandlerForResource("ReplicaSet"))
	i.informerFactory.Storage().V1().StorageClasses().Informer().AddEventHandler(i.getEventHandlerForResource("StorageClass"))
	i.informerFactory.Core().V1().Events().Informer().AddEventHandler(i.getEventHandlerForResource("Event"))
	i.informerFactory.Start(i.stopChan)
}

func (i *kubeWatcherImpl) startCustomInformers(masterURL string, kubeContext string) error {
	clientCfg := getConfig(masterURL, kubeContext)
	kubeCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to read config while starting custom informers")
	}

	crdList, err := getCrdList(kubeCfg)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(kubeCfg)
	if err != nil {
		return errors.Wrap(err, "failed to instantiate client for custom informers")
	}

	f := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, i.resync, "", nil)
	for _, crd := range crdList {
		resource, _ := schema.ParseResourceArg(crd)
		informer := f.ForResource(*resource)

		resourceKind := fmt.Sprintf("%s.%s", resource.Resource, resource.Group)
		if resource.Group == "" {
			resourceKind = resource.Resource
		}
		informer.Informer().AddEventHandler(i.getEventHandlerForResource(resourceKind))

		go func() {
			glog.V(2).Infof("Starting CRD informer for: %s (%v)", resourceKind, resource)
			informer.Informer().Run(i.stopChan)
			glog.V(2).Infof("Exited CRD informer for: %s", resourceKind)
		}()
	}

	return nil
}

func getCrdList(kubeCfg *rest.Config) ([]string, error) {
	crdClient, err := clientset.NewForConfig(kubeCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate client for querying CRDs")
	}

	crdList, err := crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to query CRDs")
	}

	glog.Infof("Found %d CRD definitions", len(crdList.Items))
	var resources []string
	for _, crd := range crdList.Items {
		resourceName := fmt.Sprintf("%s.%s.%s", crd.Spec.Names.Plural, crd.Spec.Version, crd.Spec.Group)
		glog.V(5).Infof("CRD: %s, kind: %s, plural:%s, singular:%s, short names:%v", resourceName, crd.Spec.Names.Kind, crd.Spec.Names.Plural, crd.Spec.Names.Singular, crd.Spec.Names.ShortNames)
		resources = append(resources, resourceName)
	}
	return resources, nil
}

func (i *kubeWatcherImpl) getEventHandlerForResource(resourceKind string) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    i.reportAdd(resourceKind),
		DeleteFunc: i.reportDelete(resourceKind),
		UpdateFunc: i.reportUpdate(resourceKind),
	}
}

func (i *kubeWatcherImpl) reportAdd(kind string) func(interface{}) {
	return func(obj interface{}) {
		watchResultShell := &typed.KubeWatchResult{
			Timestamp: ptypes.TimestampNow(),
			Kind:      kind,
			WatchType: typed.KubeWatchResult_ADD,
			Payload:   "",
		}
		i.processUpdate(kind, obj, watchResultShell)
	}
}

func (i *kubeWatcherImpl) reportDelete(kind string) func(interface{}) {
	return func(obj interface{}) {
		watchResultShell := &typed.KubeWatchResult{
			Timestamp: ptypes.TimestampNow(),
			Kind:      kind,
			WatchType: typed.KubeWatchResult_DELETE,
			Payload:   "",
		}
		i.processUpdate(kind, obj, watchResultShell)
	}
}

func (i *kubeWatcherImpl) reportUpdate(kind string) func(interface{}, interface{}) {
	return func(_ interface{}, newObj interface{}) {
		watchResultShell := &typed.KubeWatchResult{
			Timestamp: ptypes.TimestampNow(),
			Kind:      kind,
			WatchType: typed.KubeWatchResult_UPDATE,
			Payload:   "",
		}
		i.processUpdate(kind, newObj, watchResultShell)
	}
}

func (i *kubeWatcherImpl) processUpdate(kind string, obj interface{}, watchResult *typed.KubeWatchResult) {
	resourceJson, err := i.getResourceAsJsonString(kind, obj)
	if err != nil {
		glog.Error(err)
		return
	}

	kubeMetadata, err := kubeextractor.ExtractMetadata(resourceJson)
	if err != nil {
		// We are only grabbing namespace here for a prometheus metric, so if metadata extract fails we just log and continue
		glog.V(2).Infof("No namespace for resource: %v", err)
	}
	metricIngressKubewatchcount.WithLabelValues(kind, watchResult.WatchType.String(), kubeMetadata.Namespace).Inc()
	metricIngressKubewatchbytes.WithLabelValues(kind, watchResult.WatchType.String(), kubeMetadata.Namespace).Add(float64(len(resourceJson)))

	glog.V(5).Infof("Informer update - Name: %s, Namespace: %s, ResourceVersion: %s", kubeMetadata.Name, kubeMetadata.Namespace, kubeMetadata.ResourceVersion)
	watchResult.Payload = resourceJson
	i.writeToOutChan(watchResult)
}

func (i *kubeWatcherImpl) writeToOutChan(watchResult *typed.KubeWatchResult) {
	// We need to ensure that no messages are written to outChan after stop is called
	// Kube watch library has a way to tell it to stop, but no way to know it is complete
	// Use a lock around output channel for this purpose
	i.outchanlock.Lock()
	defer i.outchanlock.Unlock()
	if i.stopped {
		return
	}
	i.outchan <- *watchResult
}

func (i *kubeWatcherImpl) getResourceAsJsonString(kind string, obj interface{}) (string, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("resource cannot be marshalled %v", err)
	}

	return string(bytes), nil
}

func (i *kubeWatcherImpl) Stop() {
	glog.Infof("Stopping kubeWatcher")

	i.outchanlock.Lock()
	if i.stopped {
		return
	}
	i.stopped = true
	i.outchanlock.Unlock()

	close(i.stopChan)
}
