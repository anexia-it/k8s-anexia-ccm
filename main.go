package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/go-logr/logr"
	"k8s.io/component-base/config"

	"github.com/spf13/pflag"

	_ "github.com/anexia-it/k8s-anexia-ccm/anx/logging" // register human-readable console log format
	_ "github.com/anexia-it/k8s-anexia-ccm/anx/provider"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/names"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/logs/json/register"          // register optional JSON log format
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"
)

const (
	logFormatEnvironmentVariable = "LOG_FORMAT"
	logLevelEnvironmentVariable  = "LOG_LEVEL"
)

func main() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)

	ccmOptions, err := configuration.GetManagerOptions()
	if _, isSet := os.LookupEnv("DEBUG_DISABLE_LEADER_ELECTION"); isSet {
		ccmOptions.Generic.LeaderElection = config.LeaderElectionConfiguration{LeaderElect: false}
	}
	allowedPaths := append(ccmOptions.Authorization.AlwaysAllowPaths, "metrics")
	ccmOptions.Authorization.AlwaysAllowPaths = allowedPaths
	ccmOptions.SecureServing.BindPort = 8080
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	controllerInitializers := app.DefaultInitFuncConstructors

	fss := cliflag.NamedFlagSets{}
	command := app.NewCloudControllerManagerCommand(
		ccmOptions,
		cloudInitializer,
		controllerInitializers,
		names.CCMControllerAliases(),
		fss,
		wait.NeverStop,
	)
	command.SetArgs(argumentsWithLoggingEnvironment(os.Args[1:], os.LookupEnv))

	logs.InitLogs()
	defer logs.FlushLogs()
	cmdContext := logr.NewContext(context.Background(), klog.NewKlogr())
	if err := command.ExecuteContext(cmdContext); err != nil {
		os.Exit(1)
	}
}

// argumentsWithLoggingEnvironment turns environment-based logging configuration into the
// standard Kubernetes logging flags. Explicit command-line flags take precedence.
func argumentsWithLoggingEnvironment(args []string, lookupEnv func(string) (string, bool)) []string {
	environmentArgs := make([]string, 0, 2)

	if format, ok := nonEmptyEnvironmentValue(lookupEnv, logFormatEnvironmentVariable); ok &&
		!hasFlag(args, "--logging-format", "") {
		environmentArgs = append(environmentArgs, fmt.Sprintf("--logging-format=%s", format))
	}

	if level, ok := nonEmptyEnvironmentValue(lookupEnv, logLevelEnvironmentVariable); ok &&
		!hasFlag(args, "--v", "-v") {
		environmentArgs = append(environmentArgs, fmt.Sprintf("--v=%s", level))
	}

	return append(environmentArgs, args...)
}

func nonEmptyEnvironmentValue(lookupEnv func(string) (string, bool), name string) (string, bool) {
	value, ok := lookupEnv(name)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func hasFlag(args []string, longName, shortName string) bool {
	for _, argument := range args {
		if argument == longName || strings.HasPrefix(argument, longName+"=") {
			return true
		}

		if shortName != "" && (argument == shortName || strings.HasPrefix(argument, shortName+"=")) {
			return true
		}
	}

	return false
}

func cloudInitializer(config *cloudcontrollerconfig.CompletedConfig) cloudprovider.Interface {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider
	// initialize cloud provider with the cloud provider name and config file provided
	cloud, err := cloudprovider.InitCloudProvider(cloudConfig.Name, cloudConfig.CloudConfigFile)

	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}

	if cloud == nil {
		klog.Fatalf("Cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future." +
				" Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found. A ClusterID is required for the cloud provider to function properly." +
				"This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}

	return cloud
}
