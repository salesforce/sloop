/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package ingress

import (
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Variables assigned to facilitate unit testing

var BuildConfigFromFlags = clientcmd.BuildConfigFromFlags
var ClientConfig = clientcmd.ClientConfig.ClientConfig
var RawConfig = clientcmd.ClientConfig.RawConfig

// GetKubernetesContext takes optional user preferences and returns the Kubernetes context in use
func GetKubernetesContext(masterURL string, kubeContextPreference string, privilegedAccess bool) (string, error) {
	glog.Infof("Getting k8s context with user-defined config masterURL=%v, kubeContextPreference=%v.", masterURL, kubeContextPreference)
	contextInUse := kubeContextPreference
	if privilegedAccess {
		clientConfig := getConfig(masterURL, kubeContextPreference)
		// This tells us the currentContext defined in the kubeConfig which gets used if we dont have an override
		rawConfig, err := RawConfig(clientConfig)
		if err != nil {
			return "", err
		}

		contextInUse = rawConfig.CurrentContext
		if kubeContextPreference != "" {
			contextInUse = kubeContextPreference
		}
	}

	glog.Infof("Get k8s context with context=%v", contextInUse)
	return contextInUse, nil
}

// MakeKubernetesClient takes masterURL and kubeContext (user preference should have already been resolved before calling this)
// and returns a K8s client
func MakeKubernetesClient(masterURL string, kubeContext string, privilegedAccess bool) (kubernetes.Interface, error) {
	glog.Infof("Creating k8sclient with user-defined config masterURL=%v, kubeContext=%v.", masterURL, kubeContext)
	var config *rest.Config
	var err error
	if privilegedAccess {
		clientConfig := getConfig(masterURL, kubeContext)
		config, err = ClientConfig(clientConfig)
		glog.Infof("Building k8sclient with context=%v, masterURL=%v, configFile=%v.", kubeContext, config.Host, clientConfig.ConfigAccess().GetLoadingPrecedence())
	} else {
		glog.Infof("Creating Config using BuildConfigFromFlags")
		config, err = BuildConfigFromFlags(masterURL, "")
		if err != nil {
			glog.Errorf("Cannot create config using BuildConfigFromFlags")
			return nil, err
		}
		glog.Infof("Building k8sclient with context=%v, masterURL=%v.", kubeContext, config.Host)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Errorf("Cannot Initialize Kubernetes Client API: %v", err)
		return nil, err
	}

	glog.Infof("Created k8sclient with above configurations")
	return clientset, nil
}

func getConfig(masterURL string, kubeContext string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext, ClusterInfo: api.Cluster{Server: masterURL}})
}
