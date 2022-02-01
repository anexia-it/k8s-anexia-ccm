package controller

import (
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/controller/lbaas/sync"
	"k8s.io/cloud-provider/app"
)

var AnexiaDefaultInitFuncConstructors = map[string]app.ControllerInitFuncConstructor{
	"anx-lbaas-sync": {
		InitContext: app.ControllerInitContext{
			ClientName: "lbaas-sync-controller",
		},
		Constructor: sync.LoadBalancerSyncConstructor,
	},
}
