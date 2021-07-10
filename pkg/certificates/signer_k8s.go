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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	K8SSignerCAMountPath = "/etc/ssl/certs/kubernetes"
	K8SSignerCAFilename  = "ca-bundle.crt"
)

const (
	OpenshiftCSRConfigMapName = "csr-controller-ca"
	OpenshiftCSRConfigMapNS   = "openshift-config-managed"

	K8SCSRConfigMapName = "cluster-info"
	K8SCSRConfigMapNS   = "kube-public"

	K8SSignerCAConfigMapNameDefault = "csr-signer-ca"

	K8SSignerCAFilepathNoMap      = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	K8SSignerCAConfigMapNameNoMap = ""
)

var K8SSignerCAConfigMapName string = K8SSignerCAConfigMapNameDefault
var K8SSignerCAFilepath string = K8SSignerCAMountPath + "/" + K8SSignerCAFilename

type signerK8S struct {
	corev1    corev1api.CoreV1Interface
	betav1Csr beta1cert.CertificateSigningRequestInterface
}

func InitK8SCA(scheme *runtime.Scheme, owner metav1.Object) (CertificateSigner, error) {
	cl := k8s.GetCoreV1()
	caCert, err := getCA(cl)
	if err != nil {
		return nil, err
	}
	if caCert == "" {
		K8SSignerCAConfigMapName = K8SSignerCAConfigMapNameNoMap
		K8SSignerCAFilepath = K8SSignerCAFilepathNoMap
	} else {
		if err = createOrUpdateCAConfigMap(caCert, cl, scheme, owner); err != nil {
			return nil, err
		}
	}
	return getK8SSigner(cl, k8s.GetBetaV1Csr(), scheme, owner)
}

func getOpenShiftCA(cl corev1api.CoreV1Interface) (caCert string, err error) {
	var cm *corev1.ConfigMap
	if cm, err = cl.ConfigMaps(OpenshiftCSRConfigMapNS).Get(OpenshiftCSRConfigMapName, metav1.GetOptions{}); err != nil {
		return "", fmt.Errorf("failed to get CA configmap %s/%s: %+v", OpenshiftCSRConfigMapNS, OpenshiftCSRConfigMapName, err)
	}
	var ok bool
	if caCert, ok = cm.Data[K8SSignerCAFilename]; !ok || caCert == "" {
		return "", fmt.Errorf("There is no %s in configmap %s/%s: %+v", K8SSignerCAFilename, OpenshiftCSRConfigMapNS, OpenshiftCSRConfigMapName, err)
	}
	return
}

func getK8SCA(cl corev1api.CoreV1Interface) (string, error) {
	cm, err := cl.ConfigMaps(K8SCSRConfigMapNS).Get(K8SCSRConfigMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get CA configmap %s/%s: %+v", K8SCSRConfigMapNS, K8SCSRConfigMapName, err)
	}
	var kubeConfig struct {
		Clusters []struct {
			Cluster struct {
				CertificateAuthorityData string `json:"certificate-authority-data"`
			}
		}
	}
	if err = k8s.YamlToStruct(cm.Data["kubeconfig"], &kubeConfig); err != nil {
		return "", fmt.Errorf("Failed to parse kubeconfig %+v, err=%+v", kubeConfig, err)
	}
	if len(kubeConfig.Clusters) == 0 {
		return "", fmt.Errorf("No cluster info in kubeconfig %+v, err=%+v", kubeConfig, err)
	}
	if kubeConfig.Clusters[0].Cluster.CertificateAuthorityData == "" {
		return "", fmt.Errorf("Empty CertificateAuthorityData in kubeconfig %+v, err=%+v", kubeConfig, err)
	}

	var decoded []byte
	if decoded, err = base64.StdEncoding.DecodeString(kubeConfig.Clusters[0].Cluster.CertificateAuthorityData); err != nil {
		return "", fmt.Errorf("Failed to decode CA cert from kubeconfig %+v, err=%+v", kubeConfig, err)
	}
	return string(decoded), nil
}

func getCA(cl corev1api.CoreV1Interface) (caCert string, err error) {
	if caCert, err = getOpenShiftCA(cl); err != nil {
		caCert, err = getK8SCA(cl)
	}
	return
}

func createOrUpdateCAConfigMap(caCert string, cl corev1api.CoreV1Interface, scheme *runtime.Scheme, owner metav1.Object) error {
	ns := owner.GetNamespace()
	iface := cl.ConfigMaps(ns)
	cm, err := iface.Get(K8SSignerCAConfigMapName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get CA configmap %s: %w", K8SSignerCAConfigMapName, err)
	}
	if cm == nil || errors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      K8SSignerCAConfigMapName,
				Namespace: ns,
			},
			Data: map[string]string{K8SSignerCAFilename: caCert},
		}
		if err = controllerutil.SetControllerReference(owner, cm, scheme); err != nil {
			return fmt.Errorf("failed to set controller reference for CA configmap %s/%s: cm=%+v: err=%w", ns, K8SSignerCAConfigMapName, cm, err)
		}
		if _, err = iface.Create(cm); err != nil {
			err = fmt.Errorf("failed to create CA configmap %+v: %w", cm, err)
		}
	} else {
		cm.Data = map[string]string{K8SSignerCAFilename: caCert}
		if err = controllerutil.SetControllerReference(owner, cm, scheme); err != nil {
			return fmt.Errorf("failed to update controller reference for CA configmap %s/%s: cm=%+v: err=%w", ns, K8SSignerCAConfigMapName, cm, err)
		}
		if _, err = iface.Update(cm); err != nil {
			err = fmt.Errorf("failed to update CA configmap %+v: %w", cm, err)
		}
	}
	return err
}

func getK8SSigner(cl corev1api.CoreV1Interface, betav1Csr beta1cert.CertificateSigningRequestInterface, scheme *runtime.Scheme, owner metav1.Object) (CertificateSigner, error) {
	return &signerK8S{corev1: cl, betav1Csr: betav1Csr}, nil
}

// SignCertificate signs cert via k8s api
// TODO: for now it uses following fileds from certTemplate x509.Certificate:
// Subject, DNSNames, IPAddresses
// Usages has different format so, for now it is a copy.
func (s *signerK8S) SignCertificate(secret *corev1.Secret, certTemplate x509.Certificate, privateKey *rsa.PrivateKey) ([]byte, error) {
	name := "csr-" + secret.Name + "-" + certTemplate.Subject.CommonName
	if err := s.betav1Csr.Delete(name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to delete old csr %s: %w ", secret.Name, err)
	}

	csrObj, err := cert.MakeCSR(privateKey, &certTemplate.Subject, certTemplate.DNSNames, certTemplate.IPAddresses)
	if err != nil {
		return nil, fmt.Errorf("failed to make csr %s: %w", secret.Name, err)
	}

	usages := []certificates.KeyUsage{
		certificates.UsageKeyEncipherment,
		certificates.UsageDigitalSignature,
		certificates.UsageClientAuth,
		certificates.UsageServerAuth,
	}

	req, err := csr.RequestCertificate(s.betav1Csr, csrObj, name, usages, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to request csr %s: %w", secret.Name, err)
	}

	req.Status.Conditions = append(req.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:    certificates.CertificateApproved,
		Reason:  "AutoApproved",
		Message: "AutoApproved",
	})

	if req, err = s.betav1Csr.UpdateApproval(req); err != nil {
		return nil, fmt.Errorf("failed to approve csr: %w", err)
	}

	const certificateWaitTimeout = 2 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), certificateWaitTimeout)
	defer cancel()

	certPem, err := csr.WaitForCertificate(ctx, s.betav1Csr, req)
	if err != nil {
		return nil, fmt.Errorf("failed to wait signed certificate for subject %s, req: %s, err: %w", certTemplate.Subject, req, err)
	}

	return certPem, nil
}

func (s *signerK8S) Validate(cert *x509.Certificate) error {
	caCert, err := getCA(s.corev1)
	if err != nil {
		return err
	}
	if caCert == "" {
		return nil
	}
	return ValidateCert(cert, []byte(caCert))
}
