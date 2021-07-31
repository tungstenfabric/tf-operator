package certificates

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	core "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	CaSecretName               = "contrail-ca-certificate"
	SignerCAPrivateKeyFilename = "ca-priv-key.pem"

	SelfSignerCAConfigMapName = "csr-signer-ca"
	SelfSignerCAMountPath     = "/etc/ssl/certs/kubernetes"
	SelfSignerCAFilename      = "ca-bundle.crt"
	SelfSignerCAFilepath      = SelfSignerCAMountPath + "/" + SelfSignerCAFilename
)

type CACertificate struct {
	client client.Client
	owner  metav1.Object
	scheme *runtime.Scheme
}

func NewCACertificate(client client.Client, scheme *runtime.Scheme, owner metav1.Object, ownerType string) *CACertificate {
	return &CACertificate{
		client: client,
		owner:  owner,
		scheme: scheme,
	}
}

type signer struct {
	client client.Client
	owner  metav1.Object
}

func InitSelfCA(cl client.Client, scheme *runtime.Scheme, owner metav1.Object, ownerType string) (CertificateSigner, error) {
	caCertificate := NewCACertificate(cl, scheme, owner, ownerType)
	csrSignerCaConfigMap := &corev1.ConfigMap{}
	csrSignerCaConfigMap.ObjectMeta.Name = SelfSignerCAConfigMapName
	csrSignerCaConfigMap.ObjectMeta.Namespace = owner.GetNamespace()
	_, err := controllerutil.CreateOrUpdate(context.Background(), cl, csrSignerCaConfigMap, func() error {
		csrSignerCAValue, err := caCertificate.GetCaCert()
		if err != nil {
			return err
		}
		csrSignerCaConfigMap.Data = map[string]string{SelfSignerCAFilename: string(csrSignerCAValue)}
		return controllerutil.SetControllerReference(owner, csrSignerCaConfigMap, scheme)
	})
	if err != nil {
		return nil, err
	}
	return getSelfSigner(cl, owner), nil
}

func (c *CACertificate) GetCaCert() ([]byte, error) {
	secret, err := GetCaCertSecret(c.client, c.owner.GetNamespace())
	if err != nil {
		return nil, err
	}
	return secret.Data[SelfSignerCAFilename], nil
}

func getSelfSigner(cl client.Client, owner metav1.Object) CertificateSigner {
	return &signer{client: cl, owner: owner}
}

func GetCaCertSecret(cl client.Client, ns string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: CaSecretName, Namespace: ns}, secret)
	return secret, err
}

func SignCertificateSelfCA(caCertDer, caPrivateKeyDer []byte, certTemplate x509.Certificate, publicKey crypto.PublicKey) ([]byte, error) {
	caCert, err := x509.ParseCertificate(caCertDer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}
	caCertPrivKey, err := x509.ParsePKCS1PrivateKey(caPrivateKeyDer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, caCert, publicKey, caCertPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}
	certPem, err := EncodeInPemFormat(certBytes, CertificatePemType)
	if err != nil {
		return nil, fmt.Errorf("failed to encode certificate with pem format: %w", err)
	}
	return certPem, nil
}

func (s *signer) SignCertificate(_ *core.Secret, certTemplate x509.Certificate, privateKey *rsa.PrivateKey) ([]byte, error) {
	caSecret, err := GetCaCertSecret(s.client, s.owner.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s with ca cert: %w", caSecret.Name, err)
	}
	caCertBlock, err := GetAndDecodePem(caSecret.Data, SelfSignerCAFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ca cert pem: %w", err)
	}
	caCertPrivKeyBlock, err := GetAndDecodePem(caSecret.Data, SignerCAPrivateKeyFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ca priv key pem: %w", err)
	}
	return SignCertificateSelfCA(caCertBlock.Bytes, caCertPrivKeyBlock.Bytes, certTemplate, privateKey.Public())
}

func (s *signer) Validate(cert *x509.Certificate) error {
	caSecret, err := GetCaCertSecret(s.client, s.owner.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to get self CA secret %s: %w", CaSecretName, err)
	}
	return ValidateCert(cert, caSecret.Data[SelfSignerCAFilename])
}
