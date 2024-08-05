package fieldindex

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var testenv *envtest.Environment
var cfg *rest.Config
var clientset *kubernetes.Clientset

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testenv = &envtest.Environment{}

	var err error
	cfg, err = testenv.Start()
	Expect(err).NotTo(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(testenv.Stop()).To(Succeed())
})

func TestRegisterFieldIndexes(t *testing.T) {
	Describe("RegisterFieldIndexes", func() {
		It("should be able to index an object field", func() {
			By("creating the cache")
			informer, err := cache.New(cfg, cache.Options{})
			Expect(err).NotTo(HaveOccurred())

			By("register field indexes for mobius controller")
			err = RegisterFieldIndexes(context.TODO(), informer)
			Expect(err).NotTo(HaveOccurred())
		})
	})
}
