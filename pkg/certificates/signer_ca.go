package certificates

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"time"

	core "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	CaSecretName                = "contrail-ca-certificate"
	SignerCAPrivateKeyFilename  = "ca-priv-key.pem"
	caCertValidityPeriod10Years = 10 * 365 * 24 * time.Hour // 10 years
	caRootCommonName            = "tf_csr_singer"
)

var CACertKeyLength = 4096

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
	l := log.WithName("InitSelfCA")
	l.Info("Init")
	ns := owner.GetNamespace()
	caSecret, err := GetCaCertSecret(cl, ns)
	if err != nil {
		if !errors.IsNotFound(err) {
			l.Error(err, fmt.Sprintf("Failed to check secret CA %s/%s", ns, CaSecretName))
			return nil, err
		}
		caSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      CaSecretName,
			},
		}
	}
	caCertPem, ok := caSecret.Data[CAFilename]
	if !ok {
		l.Info("Generate new self CA and key")
		var caPrivKeyPem []byte
		if caCertPem, caPrivKeyPem, err = GenerateCaCertificate(caCertValidityPeriod10Years); err != nil {
			l.Error(err, "Failed to generate self CA and key")
			return nil, err
		}
		_, err = controllerutil.CreateOrUpdate(context.Background(), cl, caSecret, func() error {
			caSecret.ObjectMeta = metav1.ObjectMeta{
				Namespace: ns,
				Name:      CaSecretName,
			}
			if caSecret.Data == nil {
				caSecret.Data = make(map[string][]byte)
			}
			caSecret.Data[CAFilename] = caCertPem
			caSecret.Data[SignerCAPrivateKeyFilename] = caPrivKeyPem
			return controllerutil.SetControllerReference(owner, caSecret, scheme)
		})
		if err != nil {
			l.Error(err, fmt.Sprintf("Failed to update CA secret %s/%s", caSecret.GetNamespace(), caSecret.GetName()))
			return nil, err
		}
	}
	if err = CreateOrUpdateCAConfigMap(caCertPem, cl, scheme, owner); err != nil {
		return nil, err
	}
	return &signer{client: cl, owner: owner}, nil
}

func GetCaCertSecret(cl client.Client, ns string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := cl.Get(context.Background(), types.NamespacedName{Name: CaSecretName, Namespace: ns}, secret)
	return secret, err
}

func GenerateCaCertificateTemplateEx(cn string, validityDuration time.Duration) (x509.Certificate, *rsa.PrivateKey, error) {
	caPrivKey, err := rsa.GenerateKey(rand.Reader, CACertKeyLength)
	if err != nil {
		return x509.Certificate{}, nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	serialNumber, err := GenerateSerialNumber()
	if err != nil {
		return x509.Certificate{}, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}
	subjectKeyId, err := HashPublicKey(caPrivKey.Public())
	if err != nil {
		return x509.Certificate{}, nil, fmt.Errorf("failed to get SubjectKeyId: %w", err)
	}
	authorityKeyId := subjectKeyId

	notBefore := time.Now()
	notAfter := notBefore.Add(validityDuration)
	caCertTemplate := x509.Certificate{
		SerialNumber:          serialNumber,
		SubjectKeyId:          subjectKeyId[:],
		AuthorityKeyId:        authorityKeyId[:],
		BasicConstraintsValid: true,
		IsCA:                  true,
		Subject: pkix.Name{
			CommonName:         cn,
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

func GenerateCaCertificateTemplate(validityDuration time.Duration) (x509.Certificate, *rsa.PrivateKey, error) {
	return GenerateCaCertificateTemplateEx(caRootCommonName, validityDuration)
}

func GenerateCaCertificate(validityDuration time.Duration) ([]byte, []byte, error) {
	caCertTemplate, caPrivKey, err := GenerateCaCertificateTemplate(validityDuration)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate template: %w", err)
	}
	caCertBits, err := x509.CreateCertificate(rand.Reader, &caCertTemplate, &caCertTemplate, caPrivKey.Public(), caPrivKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}
	caCertPem, err := EncodeInPemFormat(caCertBits, CertificatePemType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate with pem format: %w", err)
	}
	caCertPrivKeyPem, err := EncodeInPemFormat(x509.MarshalPKCS1PrivateKey(caPrivKey), PrivateKeyPemType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key with pem format: %w", err)
	}
	return caCertPem, caCertPrivKeyPem, nil
}

func SignCertificateSelfCA(caCertDer, caPrivateKeyDer []byte, certTemplate x509.Certificate, publicKey crypto.PublicKey) ([]byte, []byte, error) {
	caCert, err := x509.ParseCertificate(caCertDer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}
	caCertPrivKey, err := x509.ParsePKCS1PrivateKey(caPrivateKeyDer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, caCert, publicKey, caCertPrivKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign certificate: %w", err)
	}
	certPem, err := EncodeInPemFormat(certBytes, CertificatePemType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate with pem format: %w", err)
	}
	caCertPem, err := EncodeInPemFormat(caCertDer, CertificatePemType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode ca certificate with pem format: %w", err)
	}
	return certPem, caCertPem, nil
}

func (s *signer) SignCertificate(_ *core.Secret, certTemplate x509.Certificate, privateKey *rsa.PrivateKey) ([]byte, []byte, error) {
	caSecret, err := GetCaCertSecret(s.client, s.owner.GetNamespace())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret %s with ca cert: %w", caSecret.Name, err)
	}
	caCertBlock, err := GetAndDecodePem(caSecret.Data, CAFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode ca cert pem: %w", err)
	}
	caCertPrivKeyBlock, err := GetAndDecodePem(caSecret.Data, SignerCAPrivateKeyFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode ca priv key pem: %w", err)
	}
	return SignCertificateSelfCA(caCertBlock.Bytes, caCertPrivKeyBlock.Bytes, certTemplate, privateKey.Public())
}

func getCaCert(cl client.Client, ns string) ([]byte, error) {
	caSecret, err := GetCaCertSecret(cl, ns)
	if err != nil {
		return nil, err
	}
	ca, ok := caSecret.Data[CAFilename]
	if !ok {
		return nil, fmt.Errorf("Secret %s/%s has no CA certificate %s", ns, CaSecretName, CAFilename)
	}
	return ca, nil
}

func (s *signer) ValidateCert(cert *x509.Certificate) ([]byte, error) {
	ca, err := getCaCert(s.client, s.owner.GetNamespace())
	if err != nil {
		return nil, err
	}
	return ValidateCert(cert, ca)
}
