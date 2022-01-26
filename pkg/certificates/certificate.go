package certificates

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	CAFilename      = "ca-bundle.crt"
	CAConfigMapName = "csr-signer-ca"

	certKeyLength = 2048
)

var log = logf.Log.WithName("cert")

// Certificate object
type Certificate struct {
	client              client.Client
	scheme              *runtime.Scheme
	owner               metav1.Object
	sc                  *k8s.Secret
	signer              CertificateSigner
	certificateSubjects []CertificateSubject
}

// NewCertificate creates new cert
func NewCertificate(signer CertificateSigner, cl client.Client, scheme *runtime.Scheme, owner metav1.Object, subjects []CertificateSubject, ownerType string) (*Certificate, error) {
	secretName := owner.GetName() + "-secret-certificates"
	kubernetes := k8s.New(cl, scheme)
	return &Certificate{
		client:              cl,
		scheme:              scheme,
		owner:               owner,
		sc:                  kubernetes.Secret(secretName, ownerType, owner),
		signer:              signer,
		certificateSubjects: subjects,
	}, nil
}

// EnsureExistsAndIsSigned ensures cert is signed
func (r *Certificate) EnsureExistsAndIsSigned(force bool) error {
	return r.sc.EnsureExists(r, force)
}

type CertificateSigner interface {
	SignCertificate(secret *corev1.Secret, certTemplate x509.Certificate, privateKey *rsa.PrivateKey) ([]byte, []byte, error)
	ValidateCert(cert *x509.Certificate) ([]byte, error)
}

// FillSecret fill secret with data
func (r *Certificate) FillSecret(secret *corev1.Secret, force bool) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	for _, subject := range r.certificateSubjects {
		if subject.ip == "" {
			return fmt.Errorf("%s subject IP still no available", subject.name)
		}
	}

	for _, subject := range r.certificateSubjects {
		if err := r.createCertificateForPod(subject, secret, force); err != nil {
			return err
		}
	}

	return nil
}

func GetCAConfigMap(ns string, cl client.Client) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: CAConfigMapName, Namespace: ns}, cm); err != nil {
		return nil, err
	}
	return cm, nil
}

func GetCAFromConfigMap(ns string, cl client.Client) (string, error) {
	configMap, err := GetCAConfigMap(ns, cl)
	if err != nil {
		return "", err
	}
	caCert, ok := configMap.Data[CAFilename]
	if !ok {
		return "", fmt.Errorf("Failed to get CA from configmap %s/%s", ns, CAConfigMapName)
	}
	return caCert, nil
}

func CreateOrUpdateCAConfigMap(caCert []byte, cl client.Client, scheme *runtime.Scheme, owner metav1.Object) error {
	l := log.WithName("CreateOrUpdateCAConfigMap")
	caMd5 := k8s.Md5Sum(caCert)
	cm, err := GetCAConfigMap(owner.GetNamespace(), cl)
	if err == nil && cm.Annotations["ca-md5"] == caMd5 {
		l.Info("CA not changed", "md5", caMd5)
		return nil
	}
	if errors.IsNotFound(err) {
		l.Info("Create CA", "md5", caMd5)
		cm = &corev1.ConfigMap{}
		cm.ObjectMeta.Name = CAConfigMapName
		cm.ObjectMeta.Namespace = owner.GetNamespace()
		cm.Annotations = make(map[string]string)
		cm.Data = make(map[string]string)
	} else {
		l.Info("Update CA", "md5", caMd5)
	}
	_, err = controllerutil.CreateOrUpdate(context.Background(), cl, cm, func() error {
		cm.Annotations["ca-md5"] = caMd5
		cm.Data[CAFilename] = string(caCert)
		return controllerutil.SetControllerReference(owner, cm, scheme)
	})
	if err != nil {
		l.Error(err, "Failed to create or update CA")
	}
	return err
}

func (r *Certificate) createCertificateForPod(subject CertificateSubject, secret *corev1.Secret, force bool) error {
	l := log.WithName("createCertificateForPod").WithName(subject.name)
	cm, err := GetCAConfigMap(secret.GetNamespace(), r.client)
	if err != nil {
		return err
	}
	if ok, cert := r.certInSecret(secret, subject); !force && ok {
		if secret.Annotations["ca-md5"] == cm.Annotations["ca-md5"] {
			if _, err := ValidateCert(cert, []byte(cm.Data[CAFilename])); err == nil {
				l.Info("CA not changed and Cert is valid", "ca-md5", secret.Annotations["ca-md5"])
				return nil
			} else {
				l.Info("Cert invalid", "reason", err)
			}
		} else {
			l.Info("CA changed", "configmap", cm.Annotations["ca-md5"], "secret", secret.Annotations["ca-md5"])
		}
	}
	privateKey, err := subject.getPrivKeyFromSecret(secret)
	if err != nil {
		privateKey, err = rsa.GenerateKey(rand.Reader, certKeyLength)
		if err != nil {
			return fmt.Errorf("Failed to generate private key: %w", err)
		}
	}
	certificateTemplate, err := subject.generateCertificateTemplate(privateKey)
	if err != nil {
		return fmt.Errorf("failed to generate certificate template for %s, %s: %w", subject.hostname, subject.name, err)
	}
	certPem, _, err := r.signer.SignCertificate(secret, certificateTemplate, privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign certificate for subject %+v, err=%w", subject, err)
	}
	certPrivKeyPem, _ := EncodeInPemFormat(x509.MarshalPKCS1PrivateKey(privateKey), PrivateKeyPemType)
	secret.Data[serverPrivateKeyFileName(subject)] = certPrivKeyPem
	secret.Data[serverCertificateFileName(subject)] = certPem
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations["ca-md5"] = cm.Annotations["ca-md5"]
	delete(secret.Annotations, "changed-ca-md5")
	l.Info("Secret updated", "ca md5", secret.Annotations["ca-md5"])
	return nil
}

func (r *Certificate) certInSecret(secret *corev1.Secret, subject CertificateSubject) (bool, *x509.Certificate) {
	certPem, certOk := secret.Data[serverCertificateFileName(subject)]
	_, pemOk := secret.Data[serverPrivateKeyFileName(subject)]
	if !pemOk || !certOk {
		return false, nil
	}
	certs, err := certutil.ParseCertsPEM(certPem)
	if err != nil {
		return false, nil
	}
	if len(certs) != 1 {
		return false, nil
	}
	return true, certs[0]
}

func certFilePrefix(subject CertificateSubject) string {
	if subject.clientAuth {
		return "client"
	}
	return "server"
}

func keyFilePrefix(subject CertificateSubject) string {
	return certFilePrefix(subject)
}

func serverPrivateKeyFileName(subject CertificateSubject) string {
	return fmt.Sprintf("%s-key-%s.pem", keyFilePrefix(subject), subject.ip)
}

func serverCertificateFileName(subject CertificateSubject) string {
	return fmt.Sprintf("%s-%s.crt", certFilePrefix(subject), subject.ip)
}

func isRootCA(c *x509.Certificate) bool {
	if c.Issuer.CommonName != c.Subject.CommonName {
		return false
	}
	if len(c.AuthorityKeyId) > 0 && len(c.SubjectKeyId) > 0 {
		return bytes.Equal(c.AuthorityKeyId, c.SubjectKeyId)
	}
	return true
}

func ValidateCert(cert *x509.Certificate, caCertPem []byte) ([]byte, error) {
	l := log.WithName("Validate").WithName(cert.Subject.CommonName)
	roots := x509.NewCertPool()
	intermediates := x509.NewCertPool()
	certs, err := certutil.ParseCertsPEM(caCertPem)
	if err != nil {
		return nil, err
	}
	for _, c := range certs {
		if isRootCA(c) {
			roots.AddCert(c)
		} else {
			intermediates.AddCert(c)
		}
	}
	opts := x509.VerifyOptions{
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		Roots:         roots,
		Intermediates: intermediates,
	}
	chains, err := cert.Verify(opts)
	if err != nil {
		l.Error(err, "invalid")
		return nil, fmt.Errorf("certificate for %s is invalid: %w", cert.Subject.CommonName, err)
	}

	var plainChain []*x509.Certificate
	for _, c := range chains {
		plainChain = append(plainChain, c[1:]...)
	}
	caCertChain, err := certutil.EncodeCertificates(plainChain...)
	if err != nil {
		return nil, fmt.Errorf("failed to encode ca chain: %w", err)
	}
	chainStr := ""
	for _, c := range plainChain {
		if chainStr != "" {
			chainStr = chainStr + " "
		}
		chainStr = chainStr + c.Subject.CommonName
		if len(c.AuthorityKeyId) > 0 {
			authorityKeyId := fmt.Sprintf("%X", c.AuthorityKeyId)
			chainStr = chainStr + "(" + authorityKeyId + ")"
		}
	}
	l.Info("Success", "ca md5", k8s.Md5Sum(caCertChain), "ca chain", chainStr)
	return caCertChain, nil
}

func ValidateCertPem(cert, caCertPem []byte) ([]byte, error) {
	certs, err := certutil.ParseCertsPEM(cert)
	if err != nil {
		return nil, err
	}
	return ValidateCert(certs[0], caCertPem)
}
