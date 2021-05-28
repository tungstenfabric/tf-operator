package v1alpha1

import (
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func retrieveDataIPs(pod corev1.Pod) []string {
	var altIPs []string
	altIP, _ := getPodDataIP(&pod)
	altIPs = append(altIPs, altIP)
	return altIPs
}

// EnsureCertificatesExist ensures pod cert is issued
func EnsureCertificatesExist(instance v1.Object, pods []corev1.Pod, instanceType string, cl client.Client, scheme *runtime.Scheme) error {
	domain, err := ClusterDNSDomain(cl)
	if err != nil {
		return err
	}
	altIPs := PodAlternativeIPs{Retriever: retrieveDataIPs}
	subjects := PodsCertSubjects(domain, pods, altIPs)
	crt, err := certificates.NewCertificate(cl, scheme, instance, subjects, instanceType)
	if err != nil {
		return err
	}
	return crt.EnsureExistsAndIsSigned()
}
