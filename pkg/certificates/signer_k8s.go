package certificates

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	certificates "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	beta1cert "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	corev1api "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OpenshiftCSRConfigMapName = "csr-controller-ca"
	OpenshiftCSRConfigMapNS   = "openshift-config-managed"

	K8SCSRConfigMapName = "cluster-info"
	K8SCSRConfigMapNS   = "kube-public"
)

type signerK8S struct {
	corev1    corev1api.CoreV1Interface
	betav1Csr beta1cert.CertificateSigningRequestInterface
	scheme    *runtime.Scheme
	owner     metav1.Object
}

var Now = time.Now

func InitK8SCA(cl client.Client, scheme *runtime.Scheme, owner metav1.Object) (CertificateSigner, error) {
	l := log.WithName("InitK8SCA")
	l.Info("Init")
	signer := getK8SSigner(k8s.GetCoreV1(), k8s.GetBetaV1Csr(), scheme, owner)
	caCert, ok, err := getValidatedCAWithSecrets(signer, cl)
	if err != nil {
		return nil, err
	}
	if !ok {
		if err = CreateOrUpdateCAConfigMap(caCert, cl, scheme, owner); err != nil {
			return nil, err
		}
	}
	return signer, nil
}

func ensureTestCertificatesExist(signer *signerK8S, cl client.Client) (*corev1.Secret, error) {
	l := log.WithName("ensureTestCertificatesExist")
	clientAuth := false
	subjects := []CertificateSubject{NewSubject("test", "local", "localhost", "127.0.0.1", []string{}, []string{}, clientAuth)}
	crt, err := NewCertificate(signer, cl, signer.scheme, signer.owner, subjects, "manager")
	if err != nil {
		l.Error(err, "Failed to create new test cert")
		return nil, err
	}
	if err = crt.EnsureExistsAndIsSigned(true); err != nil {
		l.Error(err, "Failed to issue test cert")
		return nil, err
	}
	return crt.sc.Secret, nil
}

var _lastCheck time.Time = time.Now()
var _checkInterval, _ = time.ParseDuration("30s")

// In case of Openshift CA is changed during deploy
// It issues additional intermediate CA signed by root and
// some certeficates appear to be signed by root and some by intemediate CA
// So, if any is signed by intermediate needs to reissue others as well
// to make state consistent
func getValidatedCAWithSecrets(signer *signerK8S, cl client.Client) ([]byte, bool, error) {
	l := log.WithName("getValidatedCAWithSecrets")
	caCert, err := getCA(k8s.GetCoreV1())
	if err != nil {
		return nil, false, err
	}
	if len(caCert) == 0 {
		return nil, false, fmt.Errorf("Failed to read K8S CA data")
	}
	ns := signer.owner.GetNamespace()
	cm, err := GetCAConfigMap(ns, cl)
	if err != nil {
		if errors.IsNotFound(err) {
			// called first time
			l.Info("First init")
			return caCert, false, nil
		}
		l.Error(err, "Failed to get CA configmap")
		return nil, false, err
	}
	// dont check too often
	if Now != nil {
		nextCheck := Now()
		if _lastCheck.Add(_checkInterval).After(nextCheck) {
			// too early to check
			return nil, true, nil
		}
		_lastCheck = nextCheck
	}
	// sign test cert
	s, err := ensureTestCertificatesExist(signer, cl)
	if err != nil {
		return nil, false, err
	}
	certs, err := cert.ParseCertsPEM(s.Data["server-127.0.0.1.crt"])
	if err != nil {
		l.Error(err, "Failed to parse test cert")
		return nil, false, err
	}
	// get valid ca chain
	chain, err := ValidateCert(certs[0], caCert)
	if err != nil {
		l.Error(err, "Test cert is invalid")
		return nil, false, err
	}
	// check if CA configmap upto date
	if _, err := ValidateCert(certs[0], []byte(cm.Data[CAFilename])); err != nil {
		l.Info("CA configmap is invalid", "reason", err)
		return chain, false, nil
	}
	// check if md5 annotaition same
	authorityKeyId := fmt.Sprintf("%X", certs[0].AuthorityKeyId)
	l.Info("CA", "cm ca md5", cm.Annotations["ca-md5"], "secret ca md5", s.Annotations["ca-md5"], "issuer", certs[0].Issuer.CommonName, "authorityKeyId", authorityKeyId)
	return chain, cm.Annotations["ca-md5"] == s.Annotations["ca-md5"], nil
}

func GetOpenShiftCA(cl corev1api.CoreV1Interface) ([]byte, error) {
	var cm *corev1.ConfigMap
	var err error
	if cm, err = cl.ConfigMaps(OpenshiftCSRConfigMapNS).Get(OpenshiftCSRConfigMapName, metav1.GetOptions{}); err != nil {
		return nil, fmt.Errorf("failed to get CA configmap %s/%s: %+v", OpenshiftCSRConfigMapNS, OpenshiftCSRConfigMapName, err)
	}
	var ok bool
	var caCert string
	if caCert, ok = cm.Data[CAFilename]; !ok || caCert == "" {
		return nil, fmt.Errorf("There is no %s in configmap %s/%s: %+v", CAFilename, OpenshiftCSRConfigMapNS, OpenshiftCSRConfigMapName, err)
	}
	return []byte(caCert), nil
}

func getK8SCA(cl corev1api.CoreV1Interface) ([]byte, error) {
	cm, err := cl.ConfigMaps(K8SCSRConfigMapNS).Get(K8SCSRConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CA configmap %s/%s: %+v", K8SCSRConfigMapNS, K8SCSRConfigMapName, err)
	}
	var kubeConfig struct {
		Clusters []struct {
			Cluster struct {
				CertificateAuthorityData string `json:"certificate-authority-data"`
			}
		}
	}
	if err = k8s.YamlToStruct(cm.Data["kubeconfig"], &kubeConfig); err != nil {
		return nil, fmt.Errorf("Failed to parse kubeconfig %+v, err=%+v", kubeConfig, err)
	}
	if len(kubeConfig.Clusters) == 0 {
		return nil, fmt.Errorf("No cluster info in kubeconfig %+v, err=%+v", kubeConfig, err)
	}
	if kubeConfig.Clusters[0].Cluster.CertificateAuthorityData == "" {
		return nil, fmt.Errorf("Empty CertificateAuthorityData in kubeconfig %+v, err=%+v", kubeConfig, err)
	}

	var decoded []byte
	if decoded, err = base64.StdEncoding.DecodeString(kubeConfig.Clusters[0].Cluster.CertificateAuthorityData); err != nil {
		return nil, fmt.Errorf("Failed to decode CA cert from kubeconfig %+v, err=%+v", kubeConfig, err)
	}
	return decoded, nil
}

func getCA(cl corev1api.CoreV1Interface) (caCert []byte, err error) {
	if caCert, err = GetOpenShiftCA(cl); err != nil {
		caCert, err = getK8SCA(cl)
	}
	return
}

func getK8SSigner(cl corev1api.CoreV1Interface, betav1Csr beta1cert.CertificateSigningRequestInterface, scheme *runtime.Scheme, owner metav1.Object) *signerK8S {
	return &signerK8S{corev1: cl, betav1Csr: betav1Csr, scheme: scheme, owner: owner}
}

func signCertificate(name string, certTemplate x509.Certificate, privateKey *rsa.PrivateKey, betav1Csr beta1cert.CertificateSigningRequestInterface) ([]byte, error) {
	csrName := "csr-" + name + "-" + certTemplate.Subject.CommonName
	if err := betav1Csr.Delete(csrName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to delete old csr %s: %w ", csrName, err)
	}

	csrObj, err := cert.MakeCSR(privateKey, &certTemplate.Subject, certTemplate.DNSNames, certTemplate.IPAddresses)
	if err != nil {
		return nil, fmt.Errorf("failed to make csr %s: %w", csrName, err)
	}

	usages := []certificates.KeyUsage{
		certificates.UsageKeyEncipherment,
		certificates.UsageDigitalSignature,
		certificates.UsageClientAuth,
		certificates.UsageServerAuth,
	}

	req, err := csr.RequestCertificate(betav1Csr, csrObj, csrName, usages, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to request csr %s: %w", csrName, err)
	}

	req.Status.Conditions = append(req.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:    certificates.CertificateApproved,
		Reason:  "AutoApproved",
		Message: "AutoApproved",
	})

	if req, err = betav1Csr.UpdateApproval(req); err != nil {
		return nil, fmt.Errorf("failed to approve csr: %w", err)
	}

	const certificateWaitTimeout = 2 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), certificateWaitTimeout)
	defer cancel()

	certPem, err := csr.WaitForCertificate(ctx, betav1Csr, req)
	if err != nil {
		return nil, fmt.Errorf("failed to wait signed certificate for subject %s, err: %w", certTemplate.Subject, err)
	}

	if err := betav1Csr.Delete(csrName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to delete csr %s: %w ", csrName, err)
	}

	return certPem, nil
}

func validateCert(corev1 corev1api.CoreV1Interface, cert *x509.Certificate) ([]byte, error) {
	l := log.WithName(fmt.Sprintf("Validate cert CN=%s, DNS=%s", cert.Subject.CommonName, cert.DNSNames))
	caCert, err := getCA(corev1)
	if err != nil {
		l.Error(err, "Failed to get CA")
		return nil, err
	}
	caChain, err := ValidateCert(cert, caCert)
	if err != nil {
		l.Info("Cert is invalid", "err", err)
		return nil, err
	}
	return caChain, nil
}

// SignCertificate signs cert via k8s api
// TODO: for now it uses following fileds from certTemplate x509.Certificate:
// Subject, DNSNames, IPAddresses
// Usages has different format so, for now it is a copy.
func (s *signerK8S) SignCertificate(secret *corev1.Secret, certTemplate x509.Certificate, privateKey *rsa.PrivateKey) ([]byte, []byte, error) {
	l := log.WithName("SignCertificate")
	l.Info("Start", "secret", secret.GetName())
	certPem, err := signCertificate(secret.GetName(), certTemplate, privateKey, s.betav1Csr)
	if err != nil {
		return nil, nil, err
	}
	certs, err := cert.ParseCertsPEM(certPem)
	if err != nil {
		return nil, nil, err
	}
	caCert, err := validateCert(s.corev1, certs[0])
	if err != nil {
		// TODO: for dbg in unittests fake csr iface looks wronlgy implements fieldselectors
		// csr, ee := s.betav1Csr.List(metav1.ListOptions{})
		// if ee != nil {
		// 	panic(ee)
		// }
		// var msg string
		// for _, i := range csr.Items {
		// 	msg = fmt.Sprintf("%s,%s\n%s\n", msg, i.Name, i.Status.Certificate)
		// }
		// panic(fmt.Errorf("secret=%s\ncsrs: %s\ncert\n%s", secret.GetName(), msg, certPem))
		return nil, nil, err
	}
	authorityKeyId := fmt.Sprintf("%X", certs[0].AuthorityKeyId)
	l.Info("Cert issued", "secret", secret.GetName(), "CN", certs[0].Subject.CommonName, "issuer", certs[0].Issuer.CommonName, "authorityKeyId", authorityKeyId)
	return certPem, caCert, nil
}

func (s *signerK8S) ValidateCert(cert *x509.Certificate) ([]byte, error) {
	return validateCert(s.corev1, cert)
}
