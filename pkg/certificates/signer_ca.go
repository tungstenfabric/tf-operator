package certificates

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	core "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	SelfSignerCAConfigMapName = "csr-signer-ca"
	SelfSignerCAMountPath     = "/etc/ssl/certs/kubernetes"
	SelfSignerCAFilename      = "ca-bundle.crt"
	SelfSignerCAFilepath      = SelfSignerCAMountPath + "/" + SelfSignerCAFilename
)

type signer struct {
	client client.Client
	owner  metav1.Object
}

func GetSelfSigner(cl client.Client, owner metav1.Object) CertificateSigner {
	return &signer{client: cl, owner: owner}
}

func GetCaCertSecret(cl client.Client, ns string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: caSecretName, Namespace: ns}, secret)
	return secret, err
}

func (s *signer) SignCertificate(_ *core.Secret, certTemplate x509.Certificate, privateKey *rsa.PrivateKey) ([]byte, error) {
	caSecret, err := GetCaCertSecret(s.client, s.owner.GetNamespace())

	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s with ca cert: %w", caSecret.Name, err)
	}

	caCertPemBlock, err := getAndDecodePem(caSecret.Data, SelfSignerCAFilename)

	if err != nil {
		return nil, fmt.Errorf("failed to decode ca cert pem: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertPemBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}

	caCertPrivKeyPemBlock, err := getAndDecodePem(caSecret.Data, signerCAPrivateKeyFilename)

	if err != nil {
		return nil, fmt.Errorf("failed to decode ca cert priv key pem: %w", err)
	}

	caCertPrivKey, err := x509.ParsePKCS1PrivateKey(caCertPrivKeyPemBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, caCert, privateKey.Public(), caCertPrivKey)

	if err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	certPem, err := encodeInPemFormat(certBytes, certificatePemType)

	if err != nil {
		return nil, fmt.Errorf("failed to encode certificate with pem format: %w", err)
	}

	return certPem, nil
}
