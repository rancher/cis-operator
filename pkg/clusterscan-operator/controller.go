package clusterscan_operator


import (
	"context"
	"time"
	"fmt"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubeapiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	"github.com/rancher/wrangler/pkg/start"
	"github.com/rancher/wrangler/pkg/crd"
	"github.com/rancher/wrangler/pkg/apply"
	batchctl "github.com/rancher/wrangler/pkg/generated/controllers/batch"
	corectl "github.com/rancher/wrangler/pkg/generated/controllers/core"
	detector "github.com/rancher/kubernetes-provider-detector"

	cisoperatorctl "github.com/prachidamle/clusterscan-operator/pkg/generated/controllers/clusterscan-operator.cattle.io"
	"github.com/prachidamle/clusterscan-operator/pkg/clusterscan-operator/clusterscan"
)

type Controller struct {
	Namespace string
	Name      string
	ClusterProvider string

	kcs *kubernetes.Clientset
	xcs *kubeapiext.Clientset
	coreFactory    *corectl.Factory
	batchFactory   *batchctl.Factory
	cisFactory *cisoperatorctl.Factory
	apply apply.Apply
}

func NewController(cfg *rest.Config, ctx context.Context, namespace, name string) (ctl *Controller, err error) {
	if cfg == nil {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	ctl = &Controller {
		Namespace: namespace,
		Name: name,
	}

	ctl.kcs, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	ctl.xcs, err = kubeapiext.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}


	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	ctl.ClusterProvider, err = detectClusterProvider(ctx, clientset)
	if err != nil {
		return nil, err
	}
	logrus.Infof("ClusterProvider detected %v", ctl.ClusterProvider)

	ctl.apply, err = apply.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	ctl.cisFactory, err = cisoperatorctl.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building clusterscan-operator NewFactoryFromConfig: %s", err.Error())
	}

	ctl.batchFactory, err = batchctl.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building batch NewFactoryFromConfig: %s", err.Error())
	}

	ctl.coreFactory, err = corectl.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building core NewFactoryFromConfig: %s", err.Error())
	}

	return ctl, nil
}


func (c *Controller) Start(ctx context.Context, threads int, resync time.Duration) error {
	// register our handlers
	if err := c.handleJobs(ctx); err != nil {
		return err
	}
	if err := c.handlePods(ctx); err != nil {
		return err
	}
	if err := c.handleClusterScans(ctx); err != nil {
		return err
	}

	return start.All(ctx, threads, c.cisFactory, c.coreFactory, c.batchFactory)
}

func (c *Controller) registerCRD(ctx context.Context) error {
	factory := crd.NewFactoryFromClientGetter(c.xcs)

	var crds []crd.CRD
	for _, crdFn := range []func() (*crd.CRD, error){
		clusterscan.CRD,
	} {
		crdef, err := crdFn()
		if err != nil {
			return err
		}
		crds = append(crds, *crdef)
	}

	return factory.BatchCreateCRDs(ctx, crds...).BatchWait()
}

func detectClusterProvider(ctx context.Context, k8sClient kubernetes.Interface) (string, error) {
	provider, err := detector.DetectProvider(ctx, k8sClient)
	if err != nil {
		return "", err
	}
	return provider, err
}