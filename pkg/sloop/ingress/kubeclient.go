/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package ingress

import (
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Returns kubeClient, currentContext, error
func MakeKubernetesClient(masterURL string, kubeContext string) (kubernetes.Interface, string, error) {
	glog.Infof("Creating k8sclient with user-defined config masterURL=%v, kubeContext=%v.", masterURL, kubeContext)

	clientConfig := getConfig(masterURL, kubeContext)

	// This tells us the currentContext defined in the kubeConfig which gets used if we dont have an override
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, "", err
	}
	contextInUse := rawConfig.CurrentContext
	if kubeContext != "" {
		contextInUse = kubeContext
	}

	config, err := clientConfig.ClientConfig()

	if err != nil {
		return nil, "", err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Errorf("Cannot Initialize Kubernetes Client API: %v", err)
		return nil, "", err
	}

	glog.Infof("Created k8sclient with context=%v, masterURL=%v, configFile=%v.", contextInUse, config.Host, clientConfig.ConfigAccess().GetLoadingPrecedence())
	return clientset, contextInUse, nil
}

func getConfig(masterURL string, kubeContext string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext, ClusterInfo: api.Cluster{Server: masterURL}})
}
