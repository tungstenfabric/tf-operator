package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAuthParametersDefault(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	cl := fake.NewFakeClientWithScheme(scheme)
	var auth = AuthParameters{}
	require.NoError(t, auth.Prepare("tf", cl))
	require.Equal(t, AuthenticationModeNoAuth, auth.AuthMode)
}

func TestAuthParametersKeystoneDefault(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	cl := fake.NewFakeClientWithScheme(scheme)
	var auth = AuthParameters{AuthMode: AuthenticationModeKeystone}
	require.NoError(t, auth.Prepare("tf", cl))
	require.Equal(t, AuthenticationModeKeystone, auth.AuthMode)
	require.Equal(t, KeystoneAuthProto, auth.KeystoneAuthParameters.AuthProtocol)
	require.Equal(t, KeystoneAuthPort, *auth.KeystoneAuthParameters.Port)
	require.Equal(t, KeystoneAuthAdminPort, *auth.KeystoneAuthParameters.AdminPort)
	require.Equal(t, KeystoneAuthRegionName, auth.KeystoneAuthParameters.Region)
	require.Equal(t, KeystoneAuthProjectDomainName, auth.KeystoneAuthParameters.ProjectDomainName)
	require.Equal(t, KeystoneAuthUserDomainName, auth.KeystoneAuthParameters.UserDomainName)
	require.Equal(t, KeystoneAuthInsecure, *auth.KeystoneAuthParameters.Insecure)
}
