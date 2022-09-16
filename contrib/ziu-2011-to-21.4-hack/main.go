package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/log/zap"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	_ "github.com/operator-framework/operator-sdk/pkg/metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/spf13/pflag"

	"github.com/tungstenfabric/tf-operator/pkg/apis"
	controller_manager "github.com/tungstenfabric/tf-operator/pkg/controller/manager"
)

var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func exitOnErr(err error) {
	if err != nil {
		log.Error(err, "Fatal error")
		os.Exit(1)
	}
}

func do() {
	//  create k8s client
	cfg := config.GetConfigOrDie()
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		namespace = "tf"
	}

	mgr, err := manager.New(cfg, manager.Options{
		Namespace:               namespace,
		MetricsBindAddress:      "0",
		LeaderElection:          true,
		LeaderElectionID:        "tf-manager-lock",
		LeaderElectionNamespace: namespace,
	})
	exitOnErr(err)

	err = apis.AddToScheme(mgr.GetScheme())
	exitOnErr(err)

	clnt, err := client.New(cfg, client.Options{})
	exitOnErr(err)

	err = controller_manager.EnableZiu2011(namespace, clnt, mgr.GetScheme(), log)
	exitOnErr(err)
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime).
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	var repeate int
	var delay int
	pflag.CommandLine.IntVar(&repeate, "repeate", 1, "number of repeats")
	pflag.CommandLine.IntVar(&delay, "delay", 5, "delay in seconds between repeats")

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

	log.Info(fmt.Sprintf("repeate %d times", repeate))
	log.Info(fmt.Sprintf("deleay between repeats %d seconds", delay))

	printVersion()

	for i := 0; i < repeate; i++ {
		log.Info("Repeate", "i", i)
		do()
		log.Info("Sleep", "seconds", delay)
		time.Sleep(time.Duration(time.Second * time.Duration(delay)))
	}

	os.Exit(0)
}
