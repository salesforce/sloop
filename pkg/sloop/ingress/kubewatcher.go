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
	"github.com/golang/glog"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"k8s.io/client-go/tools/cache"
	"sync"
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
func NewKubeWatcherSource(kubeClient kubernetes.Interface, outChan chan typed.KubeWatchResult, resync time.Duration) (KubeWatcher, error) {
	kw := &kubeWatcherImpl{resync: resync, outchanlock: &sync.Mutex{}}
	kw.stopChan = make(chan struct{})
	kw.outchan = outChan

	kw.startInformer(kubeClient, true)
	return kw, nil
}

func (i *kubeWatcherImpl) startInformer(kubeclient kubernetes.Interface, includeEvents bool) {
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
