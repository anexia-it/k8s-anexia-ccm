package main




import (
	"context"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/configuration"
	"github.com/go-logr/logr"
	"k8s.io/component-base/config"
	"k8s.io/klog/v2/klogr"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/pflag"

	_ "github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)

	ccmOptions, err := configuration.GetManagerOptions()
	if _, isSet := os.LookupEnv("DEBUG_DISABLE_LEADER_ELECTION"); isSet {
		ccmOptions.Generic.LeaderElection = config.LeaderElectionConfiguration{LeaderElect: false}
	}

	ccmOptions.SecureServing.BindPort = 8080
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}
	controllerInitializers := app.DefaultInitFuncConstructors
	fss := cliflag.NamedFlagSets{}
	command := app.NewCloudControllerManagerCommand(ccmOptions, cloudInitializer, controllerInitializers,
		fss, wait.NeverStop)

	logs.InitLogs()
	defer logs.FlushLogs()
	cmdContext := logr.NewContext(context.Background(), klogr.New())
	if err := command.ExecuteContext(cmdContext); err != nil {
		os.Exit(1)
	}
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
