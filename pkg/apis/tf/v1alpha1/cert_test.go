package v1alpha1

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/stretchr/testify/require"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	certutil "k8s.io/client-go/util/cert"

	fakeclient "k8s.io/client-go/kubernetes/fake"
)

const (
	caCommonName                = "self-signer"
	caCertValidityPeriod10Years = 10 * 365 * 24 * time.Hour // 10 years
	caCertKeyLength             = 2048
)

func generateSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}

func generateCaCertificateTemplate(validityDuration time.Duration) (x509.Certificate, *rsa.PrivateKey, error) {
	caPrivKey, err := rsa.GenerateKey(rand.Reader, caCertKeyLength)
	if err != nil {
		return x509.Certificate{}, nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	serialNumber, err := generateSerialNumber()
	if err != nil {
		return x509.Certificate{}, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(validityDuration)
	caCertTemplate := x509.Certificate{
		SerialNumber:          serialNumber,
		BasicConstraintsValid: true,
		IsCA:                  true,
		Subject: pkix.Name{
			CommonName:         caCommonName,
			Country:            []string{"US"},
			Province:           []string{"CA"},
			Locality:           []string{"Sunnyvale"},
			Organization:       []string{"LFN"},
			OrganizationalUnit: []string{"TF"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	return caCertTemplate, caPrivKey, nil
}

func generateCaCertificate(validityDuration time.Duration) ([]byte, []byte, error) {
	caCertTemplate, caPrivKey, err := generateCaCertificateTemplate(validityDuration)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate template: %w", err)
	}
	caCertBits, err := x509.CreateCertificate(rand.Reader, &caCertTemplate, &caCertTemplate, caPrivKey.Public(), caPrivKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}
	caCertPem, err := certificates.EncodeInPemFormat(caCertBits, certificates.CertificatePemType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate with pem format: %w", err)
	}
	caCertPrivKeyPem, err := certificates.EncodeInPemFormat(x509.MarshalPKCS1PrivateKey(caPrivKey), certificates.PrivateKeyPemType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key with pem format: %w", err)
	}
	return caCertPem, caCertPrivKeyPem, nil
}

func getSelfCASecret(validityDuration time.Duration) (*corev1.Secret, error) {
	caCert, caCertPrivKey, err := generateCaCertificate(validityDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ca certificate: %w", err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "tf",
			Name:      certificates.CaSecretName,
		},
		Data: map[string][]byte{
			certificates.SelfSignerCAFilename:       caCert,
			certificates.SignerCAPrivateKeyFilename: caCertPrivKey,
		},
	}
	return secret, nil
}

func init() {
	os.Setenv(k8sutil.WatchNamespaceEnvVar, "tf")
	fakeClientSet := fakeclient.NewSimpleClientset()
	k8s.SetClientset(fakeClientSet.CoreV1())
}

var owner_type string = "config"
var owner *Config = &Config{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "config1",
		Namespace: "tf",
	},
}
var pods []corev1.Pod = []corev1.Pod{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "tf",
		},
		Spec: corev1.PodSpec{
			NodeName: "node1.example.com",
		},
		Status: corev1.PodStatus{
			PodIP: "192.168.1.100",
		},
	},
}

func prepareSelfCA(t *testing.T) (*corev1.Secret, client.Client, *runtime.Scheme) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	caSecret, err := getSelfCASecret(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to create secret with self CA")
	cl := fake.NewFakeClientWithScheme(scheme, caSecret)
	require.NoError(t, InitCA(cl, scheme, owner, owner_type))
	return caSecret, cl, scheme
}

func getServerCerts(t *testing.T, cl client.Client) []*x509.Certificate {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Name: owner.Name + "-secret-certificates", Namespace: owner.Namespace}
	require.NoErrorf(t, cl.Get(context.Background(), namespacedName, secret), "Failed to get secret %+v", namespacedName)
	certBytes, ok := secret.Data["server-192.168.1.100.crt"]
	require.Equal(t, true, ok, "Failed to get server-192.168.1.100.crt from secret")
	certPEM, err := certutil.ParseCertsPEM(certBytes)
	require.NoErrorf(t, err, "Failed to parse cert %s", string(certBytes))
	return certPEM
}

func TestSelfSignedCAInit(t *testing.T) {
	_, _, _ = prepareSelfCA(t)
	require.Equal(t, certificates.SelfSignerCAConfigMapName, SignerCAConfigMapName, "Wrong SignerCAConfigMapName")
	require.Equal(t, certificates.SelfSignerCAMountPath, SignerCAMountPath, "Wrong SignerCAMountPath")
	require.Equal(t, certificates.SelfSignerCAFilename, SignerCAFilename, "Wrong SignerCAFilename")
	require.Equal(t, certificates.SelfSignerCAFilepath, SignerCAFilepath, "Wrong SignerCAFilepath")
}

func TestSelfSignedCAIssueCert(t *testing.T) {
	caSecret, cl, scheme := prepareSelfCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	for _, c := range getServerCerts(t, cl) {
		require.NoError(t, certificates.ValidateCert(c, caSecret.Data[SignerCAFilename]), "Invalid cert")
	}
}

func TestSelfSignedCARenewal(t *testing.T) {
	_, cl, scheme := prepareSelfCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	// replace root CA
	var err error
	caSecret, err := getSelfCASecret(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to create secret with self CA")
	require.NoError(t, cl.Update(context.TODO(), caSecret), "Failed to update root CA")
	// check cert is invalide now
	for _, c := range getServerCerts(t, cl) {
		require.Error(t, certificates.ValidateCert(c, caSecret.Data[SignerCAFilename]), "Validate must fail")
	}
	// replace cert
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	// check cert is valid again
	for _, c := range getServerCerts(t, cl) {
		require.NoError(t, certificates.ValidateCert(c, caSecret.Data[SignerCAFilename]), "Invalid cert")
	}
}
