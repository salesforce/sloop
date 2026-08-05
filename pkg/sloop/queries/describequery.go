/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/describe"
)

// GetResDescribe renders the stored payloads of a resource the same way `kubectl describe`
// would have at that point in time. It returns one entry per payload in the time range that
// produced a distinct describe output.
//
// kubectl's describers normally fetch live objects from an API server. Here each historical
// payload is deserialized back into its typed object and loaded into a fake clientset
// (together with the resource's Events from the watch table), and the matching describer runs
// against that. Kinds without an externally constructible describer (e.g. Deployment,
// StatefulSet, and all CRDs) fall back to a generic rendering modeled on kubectl's generic
// describer.

type DescribeOutput struct {
	PayloadKey  string `json:"payloadKey"`
	PayLoadTime int64  `json:"payloadTime"`
	Describe    string `json:"describe"`
}

func GetResDescribe(params url.Values, t typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	watchRes, err := getResPayloadWatchResults(params, t, startTime, endTime, requestId)
	if err != nil {
		return []byte{}, err
	}
	payloadOutputList := removeDupePayloads(getPayloadOutputList(watchRes))

	events, err := getResourceEventList(params, t, startTime, endTime, requestId)
	if err != nil {
		// Describe output is still useful without the events section
		glog.Errorf("Failed to get events for describe query: %v", err)
		events = nil
	}

	selectedKind := params.Get(KindParam)
	describeList := []DescribeOutput{}
	lastDescribe := ""
	for _, payloadOutput := range payloadOutputList {
		describeText, describeErr := describePayload(selectedKind, payloadOutput.Payload, events)
		if describeErr != nil {
			glog.Errorf("Failed to describe payload for key %v: %v", payloadOutput.PayloadKey, describeErr)
			describeText = fmt.Sprintf("Failed to generate describe output: %v", describeErr)
		}
		// Payloads that differ only in fields describe does not show (e.g. resourceVersion)
		// produce identical output, so collapse them the same way removeDupePayloads does
		if describeText == lastDescribe {
			continue
		}
		lastDescribe = describeText
		describeList = append(describeList, DescribeOutput{
			PayloadKey:  payloadOutput.PayloadKey,
			PayLoadTime: payloadOutput.PayLoadTime,
			Describe:    describeText,
		})
	}

	bytes, err := json.MarshalIndent(describeList, "", " ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json for DescribeList %v", err)
	}
	return bytes, nil
}

// getResourceEventList returns the Events involving the selected resource within the time
// range, deserialized to corev1.Event with only the latest stored copy of each event kept.
func getResourceEventList(params url.Values, t typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]corev1.Event, error) {
	watchEvents, err := getEventWatchResults(params, t, startTime, endTime, requestId)
	if err != nil {
		return nil, err
	}

	// The watch table holds every update of an Event (count keeps growing); keep the latest per name
	latestKeys := map[string]typed.WatchTableKey{}
	for key := range watchEvents {
		if existing, ok := latestKeys[key.Name]; !ok || key.Timestamp.After(existing.Timestamp) {
			latestKeys[key.Name] = key
		}
	}

	events := []corev1.Event{}
	for _, key := range latestKeys {
		event := corev1.Event{}
		err = json.Unmarshal([]byte(watchEvents[key].Payload), &event)
		if err != nil {
			glog.Errorf("Failed to unmarshal event payload for key %v: %v", key.String(), err)
			continue
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.Time.Before(events[j].LastTimestamp.Time)
	})
	return events, nil
}

type typedDescriberEntry struct {
	// Payloads from the well-known informers carry no apiVersion/kind (TypeMeta is empty in
	// informer objects), so each kind maps to its concrete type explicitly
	newObject    func() runtime.Object
	newDescriber func(c kubernetes.Interface) describe.ResourceDescriber
}

// Kinds from startWellKnownInformers whose kubectl describers can be constructed with a plain
// clientset. The remaining well-known kinds (Deployment, StatefulSet, HorizontalPodAutoscaler,
// MutatingWebhookConfiguration, Event) have describers with unexported fields or none at all,
// and use the generic fallback like CRDs do.
var typedDescribers = map[string]typedDescriberEntry{
	"Pod": {
		func() runtime.Object { return &corev1.Pod{} },
		func(c kubernetes.Interface) describe.ResourceDescriber { return &describe.PodDescriber{Interface: c} },
	},
	"ReplicaSet": {
		func() runtime.Object { return &appsv1.ReplicaSet{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.ReplicaSetDescriber{Interface: c}
		},
	},
	"DaemonSet": {
		func() runtime.Object { return &appsv1.DaemonSet{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.DaemonSetDescriber{Interface: c}
		},
	},
	"Job": {
		func() runtime.Object { return &batchv1.Job{} },
		func(c kubernetes.Interface) describe.ResourceDescriber { return &describe.JobDescriber{Interface: c} },
	},
	"Namespace": {
		func() runtime.Object { return &corev1.Namespace{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.NamespaceDescriber{Interface: c}
		},
	},
	"Node": {
		func() runtime.Object { return &corev1.Node{} },
		func(c kubernetes.Interface) describe.ResourceDescriber { return &describe.NodeDescriber{Interface: c} },
	},
	"PersistentVolume": {
		func() runtime.Object { return &corev1.PersistentVolume{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.PersistentVolumeDescriber{Interface: c}
		},
	},
	"PersistentVolumeClaim": {
		func() runtime.Object { return &corev1.PersistentVolumeClaim{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.PersistentVolumeClaimDescriber{Interface: c}
		},
	},
	"Service": {
		func() runtime.Object { return &corev1.Service{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.ServiceDescriber{Interface: c}
		},
	},
	// Sloop's kind label for v1.Endpoints
	"Endpoint": {
		func() runtime.Object { return &corev1.Endpoints{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.EndpointsDescriber{Interface: c}
		},
	},
	"ConfigMap": {
		func() runtime.Object { return &corev1.ConfigMap{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.ConfigMapDescriber{Interface: c}
		},
	},
	"StorageClass": {
		func() runtime.Object { return &storagev1.StorageClass{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.StorageClassDescriber{Interface: c}
		},
	},
	"PodDisruptionBudget": {
		func() runtime.Object { return &policyv1.PodDisruptionBudget{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.PodDisruptionBudgetDescriber{Interface: c}
		},
	},
	"ReplicationController": {
		func() runtime.Object { return &corev1.ReplicationController{} },
		func(c kubernetes.Interface) describe.ResourceDescriber {
			return &describe.ReplicationControllerDescriber{Interface: c}
		},
	},
}

func describePayload(kind string, payload string, events []corev1.Event) (string, error) {
	entry, ok := typedDescribers[kind]
	if !ok {
		return genericDescribePayload(payload, events)
	}

	obj := entry.newObject()
	err := json.Unmarshal([]byte(payload), obj)
	if err != nil {
		return "", errors.Wrapf(err, "payload cannot be unmarshalled into kind %v", kind)
	}
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return "", fmt.Errorf("object for kind %v has no metadata", kind)
	}

	clientObjects := []runtime.Object{obj}
	for i := range events {
		clientObjects = append(clientObjects, &events[i])
	}
	client := fake.NewSimpleClientset(clientObjects...)

	describer := entry.newDescriber(client)
	return describer.Describe(metaObj.GetNamespace(), metaObj.GetName(), describe.DescriberSettings{ShowEvents: true, ChunkSize: 500})
}

// genericDescribePayload mirrors the output of kubectl's generic describer (used by
// `kubectl describe` for CRDs), which renders the object's content as an indented field tree.
func genericDescribePayload(payload string, events []corev1.Event) (string, error) {
	var content map[string]interface{}
	err := json.Unmarshal([]byte(payload), &content)
	if err != nil {
		return "", errors.Wrap(err, "payload cannot be unmarshalled")
	}

	metadata, _ := content["metadata"].(map[string]interface{})

	buf := &bytes.Buffer{}
	out := tabwriter.NewWriter(buf, 0, 8, 2, ' ', 0)
	w := describe.NewPrefixWriter(out)
	w.Write(describe.LEVEL_0, "Name:\t%v\n", metadata["name"])
	w.Write(describe.LEVEL_0, "Namespace:\t%v\n", metadata["namespace"])
	writeStringMapMultiline(w, "Labels", metadata["labels"])
	writeStringMapMultiline(w, "Annotations", metadata["annotations"])
	writeUnstructuredContent(w, describe.LEVEL_0, content, "",
		".metadata.managedFields", ".metadata.name", ".metadata.namespace", ".metadata.labels", ".metadata.annotations")
	if len(events) > 0 {
		describe.DescribeEvents(&corev1.EventList{Items: events}, w)
	}
	err = out.Flush()
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeStringMapMultiline(w describe.PrefixWriter, title string, value interface{}) {
	m, _ := value.(map[string]interface{})
	if len(m) == 0 {
		w.Write(describe.LEVEL_0, "%s:\t<none>\n", title)
		return
	}
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i == 0 {
			w.Write(describe.LEVEL_0, "%s:\t%s=%v\n", title, k, m[k])
		} else {
			w.Write(describe.LEVEL_0, "\t%s=%v\n", k, m[k])
		}
	}
}

func writeUnstructuredContent(w describe.PrefixWriter, level int, content map[string]interface{}, skipPrefix string, skip ...string) {
	fields := []string{}
	for field := range content {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	for _, field := range fields {
		skipExpr := fmt.Sprintf("%s.%s", skipPrefix, field)
		if containsString(skip, skipExpr) {
			continue
		}
		switch typedValue := content[field].(type) {
		case map[string]interface{}:
			w.Write(level, "%s:\n", smartLabelFor(field))
			writeUnstructuredContent(w, level+1, typedValue, skipExpr, skip...)
		case []interface{}:
			w.Write(level, "%s:\n", smartLabelFor(field))
			for _, child := range typedValue {
				switch typedChild := child.(type) {
				case map[string]interface{}:
					writeUnstructuredContent(w, level+1, typedChild, skipExpr, skip...)
				default:
					w.Write(level+1, "%v\n", typedChild)
				}
			}
		default:
			w.Write(level, "%s:\t%v\n", smartLabelFor(field), typedValue)
		}
	}
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// smartLabelFor turns a camelCase field name into the spaced label kubectl shows,
// e.g. "containerStatuses" -> "Container Statuses", "apiVersion" -> "API Version".
func smartLabelFor(field string) string {
	if strings.IndexFunc(field, func(r rune) bool {
		return !unicode.IsLetter(r) && r != '-'
	}) != -1 {
		return field
	}

	commonAcronyms := map[string]bool{"API": true, "URL": true, "UID": true, "OSB": true, "GUID": true}
	result := []string{}
	for _, part := range splitCamelCase(field) {
		if commonAcronyms[strings.ToUpper(part)] {
			part = strings.ToUpper(part)
		} else {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			part = string(runes)
		}
		result = append(result, part)
	}
	return strings.Join(result, " ")
}

// splitCamelCase splits at lower-to-upper transitions and before the last upper of an
// uppercase run ("APIVersion" -> "API", "Version").
func splitCamelCase(s string) []string {
	runes := []rune(s)
	parts := []string{}
	start := 0
	for i := 1; i < len(runes); i++ {
		lowerToUpper := unicode.IsUpper(runes[i]) && !unicode.IsUpper(runes[i-1])
		endOfUpperRun := i+1 < len(runes) && unicode.IsUpper(runes[i]) && !unicode.IsUpper(runes[i+1]) && unicode.IsUpper(runes[i-1])
		if lowerToUpper || endOfUpperRun {
			parts = append(parts, string(runes[start:i]))
			start = i
		}
	}
	parts = append(parts, string(runes[start:]))
	return parts
}
