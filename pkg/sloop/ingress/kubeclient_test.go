package ingress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestGetKubernetesContext(t *testing.T) {
	oldRawConfig := RawConfig
	defer func() { RawConfig = oldRawConfig }()
	var privilegedAccess bool
	privilegedAccess = true
	methodInvoked := false
	RawConfig = func(config clientcmd.ClientConfig) (api.Config, error) {
		methodInvoked = true
		if !privilegedAccess {
			t.Errorf("Got unexpected flag")
		}
		return api.Config{}, nil
	}
	GetKubernetesContext("", "", privilegedAccess)
	if !methodInvoked {
		t.Errorf("RawConfig not invoked")
	}
}

func TestGetKubernetesContextNoPrivilegedAccess(t *testing.T) {
	var context string
	context, _ = GetKubernetesContext("", "", false)
	assert.Equal(t, context, "")
}

func TestMakeKubernetesClientByGetConfig(t *testing.T) {
	oldClientConfig := ClientConfig
	defer func() { ClientConfig = oldClientConfig }()
	var privilegedAccess bool
	privilegedAccess = true
	methodInvoked := false
	ClientConfig = func(config clientcmd.ClientConfig) (*rest.Config, error) {
		methodInvoked = true
		if !privilegedAccess {
			t.Errorf("Got unexpected flag")
		}
		return &rest.Config{}, nil
	}
	MakeKubernetesClient("", "", privilegedAccess)
	if !methodInvoked {
		t.Errorf("ClientConfig not invoked")
	}
}

func TestMakeKubernetesClientByBuildConfigFromFlag(t *testing.T) {
	oldBuildConfig := BuildConfigFromFlags
	defer func() { BuildConfigFromFlags = oldBuildConfig }()
	var privilegedAccess bool
	privilegedAccess = false
	methodInvoked := false
	BuildConfigFromFlags = func(masterUrl, kubeconfigPath string) (*rest.Config, error) {
		methodInvoked = true
		if privilegedAccess {
			t.Errorf("Got unexpected flag")
		}
		return &rest.Config{}, nil
	}
	MakeKubernetesClient("", "", privilegedAccess)
	if !methodInvoked {
		t.Errorf("BuildConfigFromFlags not invoked")
	}
}
