/*
Copyright 2019 Banzai Cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package istio

import (
	stdlog "log"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/banzaicloud/istio-operator/pkg/apis"
)

var k8sConfig *rest.Config

func init() {
	err := apiextensionsv1.AddToScheme(scheme.Scheme)
	if err != nil {
		stdlog.Fatal(err)
	}
}

func TestMain(m *testing.M) {
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "base", "crds")},
	}

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		stdlog.Fatal(err)
	}

	if k8sConfig, err = t.Start(); err != nil {
		stdlog.Fatal(err)
	}

	code := m.Run()
	if err = t.Stop(); err != nil {
		stdlog.Fatal(err)
	}
	os.Exit(code)
}

type Reconciliation struct {
	request reconcile.Request
	result  reconcile.Result
	err     error
}

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request and result to reconciliations after Reconcile is finished.
func SetupTestReconcile(logger logr.Logger, inner IstioReconciler) (IstioReconciler, chan Reconciliation) {
	reconciliations := make(chan Reconciliation)
	x := testReconciler{logger, inner, reconciliations}
	return x, reconciliations
}

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager, t *testing.T) (chan struct{}, *sync.WaitGroup) {
	stop := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := mgr.Start(stop)
		require.NoError(t, err)
	}()
	return stop, wg
}

type testReconciler struct {
	logger          logr.Logger
	inner           IstioReconciler
	reconciliations chan Reconciliation
}

var _ IstioReconciler = testReconciler{}

func (r testReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.logger.Info("Received reconcile request", "request", request)
	result, err := r.inner.Reconcile(request)
	r.logger.Info("Reconcile result", "result", result, "err", err)
	if err != nil {
		r.logger.Error(err, "reconcile failed, requeuing..")
	}
	r.reconciliations <- Reconciliation{
		request: request,
		result:  result,
		err:     err,
	}
	return result, err
}

func (r testReconciler) initWatches(watchCreatedResourcesEvents bool) error {
	return r.inner.initWatches(watchCreatedResourcesEvents)
}

func (r testReconciler) setController(ctrl controller.Controller) {
	r.inner.setController(ctrl)
}
