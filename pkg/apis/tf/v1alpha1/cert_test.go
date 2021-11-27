package v1alpha1

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strings"
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
	caIntermediateCommonName    = "self-signer-intermediate"
	caCertValidityPeriod10Years = 10 * 365 * 24 * time.Hour // 10 years
	caFileName                  = "ca-bundle.crt"
	caKeyFileName               = "ca-priv-key.pem"
)

func generateIntermediateCaCertificate(validityDuration time.Duration) ([]byte, []byte, []byte, []byte, error) {
	rootCAPem, rootKeyPem, err := certificates.GenerateCaCertificate(validityDuration)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	rootCA, _ := pem.Decode(rootCAPem)
	rootKey, _ := pem.Decode(rootKeyPem)
	caCertTemplate, caPrivKey, err := certificates.GenerateCaCertificateTemplateEx(caIntermediateCommonName, validityDuration)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	caCertPem, _, err := certificates.SignCertificateSelfCA(rootCA.Bytes, rootKey.Bytes, caCertTemplate, caPrivKey.Public())
	if err != nil {
		return nil, nil, nil, nil, err
	}
	caCertPrivKeyPem, err := certificates.EncodeInPemFormat(x509.MarshalPKCS1PrivateKey(caPrivKey), certificates.PrivateKeyPemType)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to encode private key with pem format: %w", err)
	}
	return []byte(fmt.Sprintf("%s%s", caCertPem, rootCAPem)), caCertPrivKeyPem, rootCAPem, rootKeyPem, nil
}

func getSelfCASecretEmpty() *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "tf",
			Name:      "contrail-ca-certificate",
		},
	}
	return secret
}

func getSelfCASecret(validityDuration time.Duration) (*corev1.Secret, error) {
	caCert, caCertPrivKey, err := certificates.GenerateCaCertificate(validityDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ca certificate: %w", err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "tf",
			Name:      "contrail-ca-certificate",
		},
		Data: map[string][]byte{
			caFileName:    caCert,
			caKeyFileName: caCertPrivKey,
		},
	}
	return secret, nil
}

type csrIface struct {
	v1Csr                       beta1cert.CertificateSigningRequestInterface
	caCertPem                   []byte
	caPrivateKeyPem             []byte
	caIntermediateCertPem       []byte
	caIntermediatePrivateKeyPem []byte
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
	// var l *v1beta1.CertificateSigningRequestList
	// var err error
	// if l, err = c.v1Csr.List(opts); err == nil {
	// 	if opts.FieldSelector != "" {
	// 		s := fields.ParseSelectorOrDie(opts.FieldSelector)
	// 		var items []v1beta1.CertificateSigningRequest
	// 		for _, i := range l.Items {
	// 			if _, ok := s.RequiresExactMatch(i.Name); ok {
	// 				items = append(items, i)
	// 			}
	// 		}
	// 		l.Items = items
	// 	}
	// }
	// return l, err
	return c.v1Csr.List(opts)
}
func (c *csrIface) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.v1Csr.Watch(opts)
}

func (c *csrIface) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.CertificateSigningRequest, err error) {
	result, err = c.v1Csr.Patch(name, pt, data, subresources...)
	return
}

func (c *csrIface) UpdateApproval(r *v1beta1.CertificateSigningRequest) (*v1beta1.CertificateSigningRequest, error) {
	var err error
	if r, err = c.v1Csr.UpdateApproval(r); err != nil {
		return r, err
	}
	var csrBlock *pem.Block
	csrBlock, _ = pem.Decode(r.Spec.Request)
	var csr *x509.CertificateRequest
	if csr, err = x509.ParseCertificateRequest(csrBlock.Bytes); err != nil {
		return nil, err
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(caCertValidityPeriod10Years)
	var serialNumber *big.Int
	if serialNumber, err = certificates.GenerateSerialNumber(); err != nil {
		return nil, err
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

	if r.Status.Certificate, _, err = certificates.SignCertificateSelfCA(caCertBlock.Bytes, caPrivateKeyBlock.Bytes, certificateTemplate, csr.PublicKey); err != nil {
		return nil, err
	}
	r, err = c.v1Csr.UpdateStatus(r)
	return r, err
}

func init() {
	os.Setenv(k8sutil.WatchNamespaceEnvVar, "tf")
	certificates.Now = nil
}

var csrIfaceImpl *csrIface = nil

func initApis() {
	k8s.SetDeployerTypeE(false)
	fakeClientSet := fakeclient.NewSimpleClientset()
	csrIfaceImpl = &csrIface{
		v1Csr:           fakeClientSet.CertificatesV1beta1().CertificateSigningRequests(),
		caCertPem:       nil,
		caPrivateKeyPem: nil,
	}
	k8s.SetClientset(fakeClientSet.CoreV1(), csrIfaceImpl)
}

var owner_ca_type string = "manager"
var ownerCA *Manager = &Manager{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "cluster1",
		Namespace: "tf",
	},
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

func validateCAConfigMap(t *testing.T, cl client.Client, caCert []byte) {
	cm := &corev1.ConfigMap{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: "csr-signer-ca", Namespace: "tf"}, cm)
	require.NoError(t, err)
	require.Equal(t, string(caCert), cm.Data[caFileName])
	require.Equal(t, string(cm.Annotations["ca-md5"]), k8s.Md5Sum([]byte(cm.Data[caFileName])))
}

func prepareSelfCA(t *testing.T) (*corev1.Secret, client.Client, *runtime.Scheme) {
	initApis()
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	caSecret, err := getSelfCASecret(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to create secret with self CA")
	cl := fake.NewFakeClientWithScheme(scheme, caSecret)
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	validateCAConfigMap(t, cl, caSecret.Data[caFileName])
	return caSecret, cl, scheme
}

func getServerCertsRaw(t *testing.T, cl client.Client) []byte {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Name: owner.Name + "-secret-certificates", Namespace: owner.Namespace}
	require.NoErrorf(t, cl.Get(context.Background(), namespacedName, secret), "Failed to get secret %+v", namespacedName)
	certBytes, ok := secret.Data["server-192.168.1.100.crt"]
	require.Equal(t, true, ok, "Failed to get server-192.168.1.100.crt from secret")
	return certBytes
}

func getServerCerts(t *testing.T, cl client.Client) []*x509.Certificate {
	certBytes := getServerCertsRaw(t, cl)
	certPEM, err := certutil.ParseCertsPEM(certBytes)
	require.NoErrorf(t, err, "Failed to parse cert %s", string(certBytes))
	return certPEM
}

func TestSelfSignedCAInitEmptySecret(t *testing.T) {
	initApis()
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	caSecret := getSelfCASecretEmpty()
	cl := fake.NewFakeClientWithScheme(scheme, caSecret)
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	err = cl.Get(context.TODO(), types.NamespacedName{Name: caSecret.GetName(), Namespace: caSecret.GetNamespace()}, caSecret)
	require.NoError(t, err)
	validateCAConfigMap(t, cl, caSecret.Data[caFileName])
}

func TestSelfSignedCAInitByUsersSecret(t *testing.T) {
	_, _, _ = prepareSelfCA(t)
}

func TestSelfSignedCAIssueCert(t *testing.T) {
	caSecret, cl, scheme := prepareSelfCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certs := getServerCerts(t, cl)
	require.Equal(t, 1, len(certs), "There must be one signed cert")
	for _, c := range certs {
		caCert, err := certificates.ValidateCert(c, caSecret.Data[caFileName])
		require.NoError(t, err, "Invalid cert")
		require.Equal(t, string(caSecret.Data[caFileName]), string(caCert), "Wrong returned CA")
	}
}

func TestSelfSignedCASecondIssueCert(t *testing.T) {
	_, cl, scheme := prepareSelfCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certBytes1 := getServerCertsRaw(t, cl)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certBytes2 := getServerCertsRaw(t, cl)
	require.Equal(t, string(certBytes1), string(certBytes2))
}

func TestSelfSignedCAUpdate(t *testing.T) {
	caSecret, cl, scheme := prepareSelfCA(t)
	var err error
	caSecret2, err := getSelfCASecret(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to create secret with self CA")
	require.NoError(t, cl.Update(context.TODO(), caSecret2), "Failed to update root CA")
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	require.NotEqual(t, string(caSecret.Data[caFileName]), string(caSecret2.Data[caFileName]))
	validateCAConfigMap(t, cl, caSecret2.Data[caFileName])
}

func TestSelfSignedCARenewal(t *testing.T) {
	_, cl, scheme := prepareSelfCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	// replace root CA
	caSecret, err := getSelfCASecret(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to create secret with self CA")
	require.NoError(t, cl.Update(context.TODO(), caSecret), "Failed to update root CA")
	// to update ca configmap
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	// check cert is invalide now
	certs := getServerCerts(t, cl)
	for _, c := range certs {
		_, err = certificates.ValidateCert(c, caSecret.Data[caFileName])
		require.Error(t, err, "Validate must fail")
	}
	// replace cert
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	// check cert is valid again
	certs = getServerCerts(t, cl)
	for _, c := range certs {
		caCert, err := certificates.ValidateCert(c, caSecret.Data[caFileName])
		require.NoError(t, err, "Invalid cert")
		require.Equal(t, string(caSecret.Data[caFileName]), string(caCert), "Wrong returned CA")
	}
}

func TestOpenshiftSelfCAInit(t *testing.T) {
	initApis()
	// if openshift detected self ca must be used
	k8s.SetDeployerTypeE(true)
	myScheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(myScheme), "Failed to add CoreV1 into scheme")
	cl := fake.NewFakeClientWithScheme(myScheme)
	require.NoError(t, InitCA(cl, myScheme, ownerCA, owner_ca_type))
	s, err := certificates.GetCaCertSecret(cl, "tf")
	require.NoError(t, err)
	require.NotEmpty(t, s.Data[caFileName])
	require.NotEmpty(t, s.Data[caKeyFileName])
	validateCAConfigMap(t, cl, s.Data[caFileName])
}

func getOpenShiftCAConfigMap(caCert string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csr-controller-ca",
			Namespace: "openshift-config-managed",
		},
		Data: map[string]string{
			caFileName: caCert,
		},
	}
}

func setOpenshiftCAObjects(t *testing.T) {
	ca, key, err := certificates.GenerateCaCertificate(caCertValidityPeriod10Years)
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
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	validateCAConfigMap(t, cl, csrIfaceImpl.caCertPem)
	return cl, scheme
}

func TestOpenshiftCAInit(t *testing.T) {
	cl, scheme := prepareOpenshiftCA(t)
	// second call should not change
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	validateCAConfigMap(t, cl, csrIfaceImpl.caCertPem)
}

func TestOpenshiftCAIssueCert(t *testing.T) {
	cl, scheme := prepareOpenshiftCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	// second call should not change
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	validateCAConfigMap(t, cl, csrIfaceImpl.caCertPem)
	// read and verify server cert
	certs := getServerCerts(t, cl)
	require.Equal(t, 1, len(certs), "There must be one signed cert")
	for _, c := range certs {
		caCert, err := certificates.ValidateCert(c, csrIfaceImpl.caCertPem)
		require.NoError(t, err, "Invalid cert")
		require.Equal(t, string(csrIfaceImpl.caCertPem), string(caCert), "Wrong returned CA")
	}
}

func TestOpenshiftCASecondIssueCert(t *testing.T) {
	cl, scheme := prepareOpenshiftCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certBytes1 := getServerCertsRaw(t, cl)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certBytes2 := getServerCertsRaw(t, cl)
	require.Equal(t, string(certBytes1), string(certBytes2))
}

func TestOpenshiftCARenewal(t *testing.T) {
	cl, scheme := prepareOpenshiftCA(t)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	validateCAConfigMap(t, cl, csrIfaceImpl.caCertPem)
	var oldCA []byte
	copy(oldCA, csrIfaceImpl.caCertPem)
	// replace root CA
	setOpenshiftCAObjects(t)
	tt, ee := certificates.GetOpenShiftCA(k8s.GetCoreV1())
	require.NoError(t, ee)
	require.Equal(t, string(csrIfaceImpl.caCertPem), string(tt))
	t.Logf("DBG: ocp ca\n%s", string(tt))
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	require.NotEqual(t, string(oldCA), string(csrIfaceImpl.caCertPem), "CA must be changed")
	validateCAConfigMap(t, cl, csrIfaceImpl.caCertPem)
	// check cert is invalid now
	certs := getServerCerts(t, cl)
	for _, c := range certs {
		_, err := certificates.ValidateCert(c, csrIfaceImpl.caCertPem)
		require.Error(t, err, "Invalid cert")
	}
	// replace cert
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	// check cert is valid again
	certs = getServerCerts(t, cl)
	for _, c := range certs {
		caCert, err := certificates.ValidateCert(c, csrIfaceImpl.caCertPem)
		require.NoError(t, err, "Invalid cert")
		require.Equal(t, string(csrIfaceImpl.caCertPem), string(caCert), "Wrong returned CA")
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

func setK8SCAConfigMap(t *testing.T, caCert []byte) {
	cm := getK8SCAConfigMap(caCert)
	_ = k8s.GetCoreV1().ConfigMaps(cm.GetNamespace()).Delete(cm.Name, &metav1.DeleteOptions{})
	_, err := k8s.GetCoreV1().ConfigMaps(cm.GetNamespace()).Create(cm)
	require.NoError(t, err, "Failed to create K8S CA configmap")

}

func setK8SCAObjects(t *testing.T, fakeIntermediate bool) {
	ca, key, rootCA, rootKey, err := generateIntermediateCaCertificate(caCertValidityPeriod10Years)
	require.NoError(t, err, "Failed to generate CA cert")

	if !fakeIntermediate {
		setK8SCAConfigMap(t, ca)
		csrIfaceImpl.caCertPem = make([]byte, len(ca))
		copy(csrIfaceImpl.caCertPem, ca)
		csrIfaceImpl.caPrivateKeyPem = make([]byte, len(key))
		copy(csrIfaceImpl.caPrivateKeyPem, key)
		return
	}
	// use root to sign certs
	setK8SCAConfigMap(t, rootCA)
	csrIfaceImpl.caCertPem = make([]byte, len(rootCA))
	copy(csrIfaceImpl.caCertPem, rootCA)
	csrIfaceImpl.caPrivateKeyPem = make([]byte, len(rootKey))
	copy(csrIfaceImpl.caPrivateKeyPem, rootKey)

	// save intermediate for checks
	csrIfaceImpl.caIntermediateCertPem = make([]byte, len(ca))
	copy(csrIfaceImpl.caIntermediateCertPem, ca)
	csrIfaceImpl.caIntermediatePrivateKeyPem = make([]byte, len(key))
	copy(csrIfaceImpl.caIntermediatePrivateKeyPem, key)
}

func prepareK8SCA(t *testing.T, fakeIntermediate bool) (client.Client, *runtime.Scheme) {
	initApis()
	setK8SCAObjects(t, fakeIntermediate)
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")
	cl := fake.NewFakeClientWithScheme(scheme)
	require.NoError(t, InitCA(cl, scheme, ownerCA, owner_ca_type))
	return cl, scheme
}

func TestK8SCAInit(t *testing.T) {
	_, _ = prepareK8SCA(t, false)
}

func TestK8SCAIssueCert(t *testing.T) {
	cl, scheme := prepareK8SCA(t, false)
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certs := getServerCerts(t, cl)
	require.Equal(t, 1, len(certs), "There must be one signed cert")
	for _, c := range certs {
		caCert, err := certificates.ValidateCert(c, csrIfaceImpl.caCertPem)
		require.NoError(t, err, "Invalid cert")
		require.Equal(t, string(csrIfaceImpl.caCertPem), string(caCert))
	}
}

func TestK8SUnusedIntermediateCA(t *testing.T) {
	cl, scheme := prepareK8SCA(t, true)
	t.Logf("DBG: root\n%s", csrIfaceImpl.caCertPem)
	t.Logf("DBG: intermediate\n%s", csrIfaceImpl.caIntermediateCertPem)
	// validate intermediate cert
	require.NotEqual(t, string(csrIfaceImpl.caCertPem), string(csrIfaceImpl.caIntermediateCertPem))
	icas, err := certutil.ParseCertsPEM(csrIfaceImpl.caIntermediateCertPem)
	require.NoError(t, err)
	require.Equal(t, 2, len(icas))
	caCert, err := certificates.ValidateCert(icas[0], csrIfaceImpl.caCertPem)
	require.NoError(t, err, "Invalid cert")
	require.Equal(t, string(csrIfaceImpl.caCertPem), string(caCert))
	// validate ca configmap
	validateCAConfigMap(t, cl, csrIfaceImpl.caCertPem)
	// check server certificate is validated by root and intermediate and in
	// case of intermediate returned valid chain is equal root
	require.NoError(t, EnsureCertificatesExist(owner, pods, owner_type, cl, scheme), "Failed to issue cert")
	certs := getServerCerts(t, cl)
	require.Equal(t, 1, len(certs), "There must be one signed cert")
	for _, c := range certs {
		caCert, err = certificates.ValidateCert(c, csrIfaceImpl.caCertPem)
		require.NoError(t, err, "Invalid cert")
		require.Equal(t, string(csrIfaceImpl.caCertPem), string(caCert))
		caCert, err = certificates.ValidateCert(c, csrIfaceImpl.caIntermediateCertPem)
		require.NoError(t, err, "Invalid cert")
		require.Equal(t, string(csrIfaceImpl.caCertPem), string(caCert))
	}
	caMd5 := k8s.Md5Sum([]byte(csrIfaceImpl.caCertPem))
	secretsList := &corev1.SecretList{}
	err = cl.List(context.TODO(), secretsList, &client.ListOptions{Namespace: "tf"})
	require.NoError(t, err)
	checked := 0
	for _, s := range secretsList.Items {
		if !strings.HasSuffix(s.Name, "secret-certificates") {
			continue
		}
		checked = checked + 1
		require.Equal(t, caMd5, s.Annotations["ca-md5"])
	}
	require.Equal(t, checked, 1)
}
