package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	_ "github.com/operator-framework/operator-sdk/pkg/metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	"k8s.io/client-go/discovery"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/tungstenfabric/tf-operator/pkg/apis"
	cert "github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/controller"
	"github.com/tungstenfabric/tf-operator/pkg/controller/kubemanager"
)

var log = logf.Log.WithName("cmd")

func setClientSignerName(major, minor int) {
	if signer, ok := os.LookupEnv("CLIENT_SIGNER_NAME"); ok {
		cert.ClientSignerName = signer
	} else if major > 1 || minor > 21 || k8s.IsOpenshift() {
		cert.ClientSignerName = cert.SelfSigner
	}
	log.Info(fmt.Sprintf("ClientSignerName: '%s'", cert.ClientSignerName))
}

func setServerSignerName(major, minor int) {
	if signer, ok := os.LookupEnv("SERVER_SIGNER_NAME"); ok {
		cert.ServerSignerName = signer
	} else if major > 1 || minor > 21 || k8s.IsOpenshift() {
		cert.ServerSignerName = cert.SelfSigner
	}
	log.Info(fmt.Sprintf("ServerSignerName: '%s'", cert.ServerSignerName))
}

func setSignerName(clnt *discovery.DiscoveryClient) error {
	if ver, err := clnt.ServerVersion(); err == nil {
		major, _ := strconv.Atoi(ver.Major)
		minor, _ := strconv.Atoi(ver.Minor)
		log.Info(fmt.Sprintf("K8S Server Version: %d.%d", major, minor))
		setClientSignerName(major, minor)
		setServerSignerName(major, minor)
		if cert.ClientSignerName != cert.ServerSignerName &&
			(cert.ClientSignerName == cert.SelfSigner || cert.ClientSignerName == cert.ExternalSigner) {
			return fmt.Errorf("Client and Server signers mismatch client=%s server=%s",
				cert.ClientSignerName, cert.ServerSignerName)
		}
	} else {
		return err
	}
	return nil
}

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func runOperator(sigHandler <-chan struct{}) (err error) {

	// Get a config to talk to the apiserver.
	cfg := config.GetConfigOrDie()

	var namespace string
	if namespace, err = k8sutil.GetWatchNamespace(); err != nil {
		log.Error(err, "Failed to get watch namespace")
		return err
	}

	// Create a new Cmd to provide shared dependencies and start components.
	var mgr manager.Manager
	if mgr, err = manager.New(cfg, manager.Options{
		Namespace:               namespace,
		MetricsBindAddress:      "0",
		LeaderElection:          true,
		LeaderElectionID:        "tf-manager-lock",
		LeaderElectionNamespace: namespace,
	}); err != nil {
		log.Error(err, "Failed create Manager instance")
		return err
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources.
	if err = apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		return err
	}

	var clnt client.Client
	if clnt, err = client.New(cfg, client.Options{}); err != nil {
		log.Error(err, "Failed to create client")
		return err
	}

	if err = k8s.SetDeployerType(clnt); err != nil {
		log.Error(err, "Failed SetDeployerType()")
		return err
	}
	log.Info("IsOpenshift=" + strconv.FormatBool(k8s.IsOpenshift()))

	dclnt := discovery.NewDiscoveryClientForConfigOrDie(cfg)
	if err := setSignerName(dclnt); err != nil {
		log.Error(err, "Failed set signer")
		return err
	}

	// Check is ZIU Required?
	if f, err := v1alpha1.IsZiuRequired(clnt); err != nil {
		log.Error(err, "try to check if ziu required")
		return err
	} else {
		if f {
			// We start ZIU process
			log.Info("Start ZIU process")
			err = v1alpha1.InitZiu(clnt)
		} else {
			// We not needed ZIU
			log.Info("ZIU not needed")
			err = v1alpha1.SetZiuStage(-1, clnt)
		}
		if err != nil {
			log.Error(err, "Failed to Set ZIU Stage")
			return err
		}
	}

	// Setup all Controllers.
	if err = controller.AddToManager(mgr); err != nil {
		log.Error(err, "")
		return err
	}

	if err = kubemanager.Add(mgr); err != nil {
		log.Error(err, "")
		return err
	}

	log.Info("Starting")
	return mgr.Start(sigHandler)
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime).
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.Logger())

	printVersion()

	sigHandler := signals.SetupSignalHandler()

	// Start the Cmd
	for {
		if err := runOperator(sigHandler); err != nil {
			delay := time.Duration(rand.Intn(5)) * time.Second
			log.Error(err, fmt.Sprintf("Manager exited non-zero.. retry in %s sec", delay))
			time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
			continue
		}
		break
	}

}
