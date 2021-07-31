package v1alpha1

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/stretchr/testify/require"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	"k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	beta1cert "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
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

type csrIface struct {
	v1Csr           beta1cert.CertificateSigningRequestInterface
	caCertPem       []byte
	caPrivateKeyPem []byte
}

func (c *csrIface) Create(r *v1beta1.CertificateSigningRequest) (*v1beta1.CertificateSigningRequest, error) {
	return c.v1Csr.Create(r)
}

func (c *csrIface) Update(r *v1beta1.CertificateSigningRequest) (*v1beta1.CertificateSigningRequest, error) {
	return c.v1Csr.Update(r)
}

func (c *csrIface) UpdateStatus(r *v1beta1.CertificateSigningRequest) (*v1beta1.CertificateSigningRequest, error) {
	return c.v1Csr.UpdateStatus(r)
}

func (c *csrIface) Delete(name string, options *metav1.DeleteOptions) error {
	return c.v1Csr.Delete(name, options)
}

func (c *csrIface) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return c.v1Csr.DeleteCollection(options, listOptions)
}

func (c *csrIface) Get(name string, options metav1.GetOptions) (*v1beta1.CertificateSigningRequest, error) {
	return c.v1Csr.Get(name, options)
}
func (c *csrIface) List(opts metav1.ListOptions) (*v1beta1.CertificateSigningRequestList, error) {
	return c.v1Csr.List(opts)
}
func (c *csrIface) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.v1Csr.Watch(opts)
}

func (c *csrIface) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.CertificateSigningRequest, err error) {
	result, err = c.v1Csr.Patch(name, pt, data, subresources...)
	return
}

func (c *csrIface) UpdateApproval(r *v1beta1.CertificateSigningRequest) (result *v1beta1.CertificateSigningRequest, err error) {
	if result, err = c.v1Csr.UpdateApproval(r); err != nil {
		return
	}
	var csrBlock *pem.Block
	csrBlock, _ = pem.Decode(result.Spec.Request)
	var csr *x509.CertificateRequest
	if csr, err = x509.ParseCertificateRequest(csrBlock.Bytes); err != nil {
		return
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(caCertValidityPeriod10Years)
	var serialNumber *big.Int
	if serialNumber, err = certificates.GenerateSerialNumber(); err != nil {
		return
	}
	certificateTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      csr.Subject,
		DNSNames:     csr.DNSNames,
		IPAddresses:  csr.IPAddresses,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		// todo: read from csr (requries conversion)
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	caCertBlock, _ := pem.Decode(c.caCertPem)
	caPrivateKeyBlock, _ := pem.Decode(c.caPrivateKeyPem)
	if result.Status.Certificate, err = certificates.SignCertificateSelfCA(caCertBlock.Bytes, caPrivateKeyBlock.Bytes, certificateTemplate, csr.PublicKey); err != nil {
		return
	}
	result, err = c.v1Csr.UpdateStatus(result)
	return
}

func init() {
	os.Setenv(k8sutil.WatchNamespaceEnvVar, "tf")
}

var csrIfaceImpl *csrIface = nil

func initApis() {
	fakeClientSet := fakeclient.NewSimpleClientset()
	csrIfaceImpl = &csrIface{
		v1Csr:           fakeClientSet.CertificatesV1beta1().CertificateSigningRequests(),
		caCertPem:       nil,
		caPrivateKeyPem: nil,
	}
	k8s.SetClientset(fakeClientSet.CoreV1(), csrIfaceImpl)
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
	initApis()
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	caSecret, err := getSelfCASecret(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to create secret with self CA")
	cl := fake.NewFakeClientWithScheme(scheme, caSecret)
	require.NoError(t, InitCA(cl, scheme, owner, owner_type))
	return caSecret, cl, scheme
}

func getServerCertRaw(t *testing.T, cl client.Client) []byte {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Name: owner.Name + "-secret-certificates", Namespace: owner.Namespace}
	require.NoErrorf(t, cl.Get(context.Background(), namespacedName, secret), "Failed to get secret %+v", namespacedName)
	certBytes, ok := secret.Data["server-192.168.1.100.crt"]
	require.Equal(t, true, ok, "Failed to get server-192.168.1.100.crt from secret")
	return certBytes
}

func getServerCerts(t *testing.T, cl client.Client) []*x509.Certificate {
	certBytes := getServerCertRaw(t, cl)
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
	certs := getServerCerts(t, cl)
	require.Equal(t, 1, len(certs), "There must be one signed cert")
	for _, c := range certs {
		require.NoError(t, certificates.ValidateCert(c, caSecret.Data[SignerCAFilename]), "Invalid cert")
	}
}

func TestSelfSignedCASecondIssueCert(t *testing.T) {
	_, cl, scheme := prepareSelfCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certBytes1 := getServerCertRaw(t, cl)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certBytes2 := getServerCertRaw(t, cl)
	require.Equal(t, certBytes1, certBytes2)
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

func getOpenShiftCAConfigMap(caCert string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csr-controller-ca",
			Namespace: "openshift-config-managed",
		},
		Data: map[string]string{
			"ca-bundle.crt": caCert,
		},
	}
}

// func getOpenShiftCASecret(caCert, caCertPrivKey []byte) *corev1.Secret {
// 	return &corev1.Secret{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Namespace: "openshift-kube-controller-manager",
// 			Name:      "csr-signer",
// 		},
// 		Type: corev1.SecretTypeTLS,
// 		Data: map[string][]byte{
// 			corev1.TLSCertKey:       caCert,
// 			corev1.TLSPrivateKeyKey: caCertPrivKey,
// 		},
// 	}
// }

func setOpenshiftCAObjects(t *testing.T) {
	ca, key, err := generateCaCertificate(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to generate CA cert")

	cm := getOpenShiftCAConfigMap(string(ca))
	_ = k8s.GetCoreV1().ConfigMaps(cm.GetNamespace()).Delete(cm.Name, &metav1.DeleteOptions{})
	_, err = k8s.GetCoreV1().ConfigMaps(cm.GetNamespace()).Create(cm)
	require.NoError(t, err, "Failed to create Openshift CA configmap")

	csrIfaceImpl.caCertPem = make([]byte, len(ca))
	copy(csrIfaceImpl.caCertPem, ca)
	csrIfaceImpl.caPrivateKeyPem = make([]byte, len(key))
	copy(csrIfaceImpl.caPrivateKeyPem, key)
}

func prepareOpenshiftCA(t *testing.T) (client.Client, *runtime.Scheme) {
	initApis()
	setOpenshiftCAObjects(t)
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	cl := fake.NewFakeClientWithScheme(scheme)
	require.NoError(t, InitCA(cl, scheme, owner, owner_type))
	return cl, scheme
}

func TestOpenshiftCAInit(t *testing.T) {
	_, _ = prepareOpenshiftCA(t)
	require.Equal(t, certificates.K8SSignerCAConfigMapName, SignerCAConfigMapName, "Wrong SignerCAConfigMapName")
	require.Equal(t, certificates.K8SSignerCAMountPath, SignerCAMountPath, "Wrong SignerCAMountPath")
	require.Equal(t, certificates.K8SSignerCAFilename, SignerCAFilename, "Wrong SignerCAFilename")
	require.Equal(t, certificates.K8SSignerCAFilepath, SignerCAFilepath, "Wrong SignerCAFilepath")
}

func TestOpenshiftCAIssueCert(t *testing.T) {
	cl, scheme := prepareOpenshiftCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certs := getServerCerts(t, cl)
	require.Equal(t, 1, len(certs), "There must be one signed cert")
	for _, c := range certs {
		require.NoError(t, certificates.ValidateCert(c, csrIfaceImpl.caCertPem), "Invalid cert")
	}
}

func TestOpenshiftCASecondIssueCert(t *testing.T) {
	cl, scheme := prepareOpenshiftCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certBytes1 := getServerCertRaw(t, cl)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certBytes2 := getServerCertRaw(t, cl)
	require.Equal(t, certBytes1, certBytes2)
}

func TestOpenshiftCARenewal(t *testing.T) {
	cl, scheme := prepareOpenshiftCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	var oldCA []byte
	copy(oldCA, csrIfaceImpl.caCertPem)
	// replace root CA
	setOpenshiftCAObjects(t)
	require.NotEqual(t, oldCA, csrIfaceImpl.caCertPem, "CA must be changed")
	// check cert is invalide now
	for _, c := range getServerCerts(t, cl) {
		require.Error(t, certificates.ValidateCert(c, csrIfaceImpl.caCertPem), "Invalid cert")
	}
	// replace cert
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	// check cert is valid again
	for _, c := range getServerCerts(t, cl) {
		require.NoError(t, certificates.ValidateCert(c, csrIfaceImpl.caCertPem), "Invalid cert")
	}
}

func getK8SCAConfigMap(caCert []byte) *corev1.ConfigMap {
	config := fmt.Sprintf(`clusters:
- cluster:
    certificate-authority-data: %s`, base64.StdEncoding.EncodeToString(caCert))
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-info",
			Namespace: "kube-public",
		},
		Data: map[string]string{
			"kubeconfig": config,
		},
	}
}

func setK8SCAObjects(t *testing.T) {
	ca, key, err := generateCaCertificate(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to generate CA cert")

	cm := getK8SCAConfigMap(ca)
	_ = k8s.GetCoreV1().ConfigMaps(cm.GetNamespace()).Delete(cm.Name, &metav1.DeleteOptions{})
	_, err = k8s.GetCoreV1().ConfigMaps(cm.GetNamespace()).Create(cm)
	require.NoError(t, err, "Failed to create K8S CA configmap")

	csrIfaceImpl.caCertPem = make([]byte, len(ca))
	copy(csrIfaceImpl.caCertPem, ca)
	csrIfaceImpl.caPrivateKeyPem = make([]byte, len(key))
	copy(csrIfaceImpl.caPrivateKeyPem, key)
}

func prepareK8SCA(t *testing.T) (client.Client, *runtime.Scheme) {
	initApis()
	setK8SCAObjects(t)
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	cl := fake.NewFakeClientWithScheme(scheme)
	require.NoError(t, InitCA(cl, scheme, owner, owner_type))
	return cl, scheme
}

func TestK8SCAInit(t *testing.T) {
	_, _ = prepareK8SCA(t)
	require.Equal(t, certificates.K8SSignerCAConfigMapName, SignerCAConfigMapName, "Wrong SignerCAConfigMapName")
	require.Equal(t, certificates.K8SSignerCAMountPath, SignerCAMountPath, "Wrong SignerCAMountPath")
	require.Equal(t, certificates.K8SSignerCAFilename, SignerCAFilename, "Wrong SignerCAFilename")
	require.Equal(t, certificates.K8SSignerCAFilepath, SignerCAFilepath, "Wrong SignerCAFilepath")
}

func TestK8SCAIssueCert(t *testing.T) {
	cl, scheme := prepareK8SCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certs := getServerCerts(t, cl)
	require.Equal(t, 1, len(certs), "There must be one signed cert")
	for _, c := range certs {
		require.NoError(t, certificates.ValidateCert(c, csrIfaceImpl.caCertPem), "Invalid cert")
	}
}
