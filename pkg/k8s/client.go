package k8s

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	beta1cert "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	corev1api "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

var lock = sync.Mutex{}

// var clientset *kubernetes.Clientset
var clientset *kubernetes.Clientset
var coreAPI corev1api.CoreV1Interface
var betav1Csr beta1cert.CertificateSigningRequestInterface

// Allows to overwrite clientset for unittests
func SetClientset(c corev1api.CoreV1Interface) {
	lock.Lock()
	defer lock.Unlock()
	coreAPI = c
	// clientset = c
}

// GetClientConfig first tries to get a config object which uses the service account kubernetes gives to pods,
// if it is called from a process running in a kubernetes environment.
// Otherwise, it tries to build config from a default kubeconfig filepath if it fails, it fallback to the default config.
// Once it get the config, it returns the same.
func getClientConfig() *rest.Config {
	config, err := rest.InClusterConfig()
	if err != nil {
		err1 := err
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(fmt.Errorf("InClusterConfig as well as BuildConfigFromFlags Failed. Error in InClusterConfig: %+v\nError in BuildConfigFromFlags: %+v", err1, err))
		}
	}
	return config
}

func getClientset() *kubernetes.Clientset {
	if clientset == nil {
		clientset = kubernetes.NewForConfigOrDie(getClientConfig())
	}
	return clientset
}

// GetCoreV1 first tries to get a config object which uses the service account kubernetes gives to pods,
func GetCoreV1() corev1api.CoreV1Interface {
	lock.Lock()
	defer lock.Unlock()
	if coreAPI == nil {
		coreAPI = getClientset().CoreV1()
	}
	return coreAPI
}

func GetBetaV1Csr() beta1cert.CertificateSigningRequestInterface {
	lock.Lock()
	defer lock.Unlock()
	if betav1Csr == nil {
		betav1Csr = getClientset().CertificatesV1beta1().CertificateSigningRequests()
	}
	return betav1Csr
}

// ExecToPodThroughAPI uninterractively exec to the pod with the command specified.
// :param string command: list of the str which specify the command.
// :param string pod_name: Pod name
// :param string namespace: namespace of the Pod.
// :param io.Reader stdin: Standerd Input if necessary, otherwise `nil`
// :return: string: Output of the command. (STDOUT)
//          string: Errors. (STDERR)
//           error: If any error has occurred otherwise `nil`
func ExecToPodThroughAPI(command []string, containerName, podName, namespace string, stdin io.Reader) (string, string, error) {
	req := GetCoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return "", "", fmt.Errorf("error adding to scheme: %v", err)
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		//Command:   strings.Fields(command),
		Command:   command,
		Container: containerName,
		Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(getClientConfig(), "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return "", "", fmt.Errorf("error in Stream: %v", err)
	}

	return stdout.String(), stderr.String(), nil
}
