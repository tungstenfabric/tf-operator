package certificates

import (
	"crypto/rand"
	"crypto/x509"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tungstenfabric/tf-operator/pkg/k8s"
)

const (
	caSecretName               = "contrail-ca-certificate"
	signerCAPrivateKeyFilename = "ca-priv-key.pem"
)

type CACertificate struct {
	client client.Client
	owner  metav1.Object
	scheme *runtime.Scheme
	secret caCertSecret
}

func NewCACertificate(client client.Client, scheme *runtime.Scheme, owner metav1.Object, ownerType string) *CACertificate {
	kubernetes := k8s.New(client, scheme)
	return &CACertificate{
		client: client,
		owner:  owner,
		scheme: scheme,
		secret: caCertSecret{kubernetes.Secret(caSecretName, ownerType, owner)},
	}
}

func (c *CACertificate) EnsureExists() error {
	return c.secret.ensureExists()
}

func (c *CACertificate) GetCaCert() ([]byte, error) {
	secret, err := GetCaCertSecret(c.client, c.owner.GetNamespace())
	if err != nil {
		return nil, err
	}
	return secret.Data[SelfSignerCAFilename], nil
}

type caCertSecret struct {
	sc *k8s.Secret
}

func (s caCertSecret) ensureExists() error {
	return s.sc.EnsureExists(s)
}

func (caCertSecret) FillSecret(secret *corev1.Secret) error {
	if caCertExistsInSecret(secret) {
		return nil
	}

	caCert, caCertPrivKey, err := generateCaCertificate()
	if err != nil {
		return fmt.Errorf("failed to generate ca certificate: %w", err)
	}

	secret.Data = map[string][]byte{
		SelfSignerCAFilename:       caCert,
		signerCAPrivateKeyFilename: caCertPrivKey,
	}
	return nil
}

func caCertExistsInSecret(secret *corev1.Secret) bool {
	if secret.Data == nil {
		return false
	}
	_, certOk := secret.Data[SelfSignerCAFilename]
	_, privKeyOk := secret.Data[signerCAPrivateKeyFilename]
	return certOk && privKeyOk
}

func generateCaCertificate() ([]byte, []byte, error) {
	caCertTemplate, caPrivKey, err := generateCaCertificateTemplate()

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate template: %w", err)
	}

	caCertBits, err := x509.CreateCertificate(rand.Reader, &caCertTemplate, &caCertTemplate, caPrivKey.Public(), caPrivKey)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	caCertPem, err := encodeInPemFormat(caCertBits, certificatePemType)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate with pem format: %w", err)
	}

	caCertPrivKeyPem, err := encodeInPemFormat(x509.MarshalPKCS1PrivateKey(caPrivKey), privateKeyPemType)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key with pem format: %w", err)
	}

	return caCertPem, caCertPrivKeyPem, nil
}
