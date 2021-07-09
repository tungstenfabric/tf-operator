package v1alpha1

import (
	"fmt"
	"sync"

	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _Lock sync.Mutex
var SignerCAConfigMapName string
var SignerCAMountPath string
var SignerCAFilename string
var SignerCAFilepath string

var signer certificates.CertificateSigner = nil

func InitCA(cl client.Client, scheme *runtime.Scheme, owner metav1.Object, ownerType string) error {
	// This might be called from reconsiles.. need sync
	_Lock.Lock()
	defer _Lock.Unlock()
	var err error
	if _, err = certificates.GetCaCertSecret(cl, owner.GetNamespace()); k8serrors.IsNotFound(err) {
		if signer, err = certificates.InitK8SCA(scheme, owner); err != nil {
			return err
		}
		SignerCAConfigMapName = certificates.K8SSignerCAConfigMapName
		SignerCAMountPath = certificates.K8SSignerCAMountPath
		SignerCAFilename = certificates.K8SSignerCAFilename
		SignerCAFilepath = certificates.K8SSignerCAFilepath
	} else {
		if signer, err = certificates.InitSelfCA(cl, scheme, owner, ownerType); err != nil {
			return err
		}
		SignerCAConfigMapName = certificates.SelfSignerCAConfigMapName
		SignerCAMountPath = certificates.SelfSignerCAMountPath
		SignerCAFilename = certificates.SelfSignerCAFilename
		SignerCAFilepath = certificates.SelfSignerCAFilepath
	}
	return err
}

func retrieveDataIPs(pod corev1.Pod) []string {
	var altIPs []string
	altIP, _ := getPodDataIP(&pod)
	altIPs = append(altIPs, altIP)
	return altIPs
}

// EnsureCertificatesExist ensures pod cert is issued
func EnsureCertificatesExist(instance v1.Object, pods []corev1.Pod, instanceType string, cl client.Client, scheme *runtime.Scheme) error {
	// This might be called from reconsiles.. need sync
	_Lock.Lock()
	defer _Lock.Unlock()
	if signer == nil {
		return fmt.Errorf("CA Signer is not initilized")
	}
	domain, err := ClusterDNSDomain(cl)
	if err != nil {
		return err
	}
	altIPs := PodAlternativeIPs{Retriever: retrieveDataIPs}
	subjects := PodsCertSubjects(domain, pods, altIPs)
	crt, err := certificates.NewCertificate(signer, cl, scheme, instance, subjects, instanceType)
	if err != nil {
		return err
	}
	return crt.EnsureExistsAndIsSigned()
}
