package store

// Create basic test harness to initialize a NewCacheStores object and then create a store object
// and add it to the cacheStores map

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDriver(t *testing.T) {
	// // create a fake logger to pass into the cachestore
	logger := logr.New(logr.Discard().GetSink())
	// // create a new CacheStores object
	// cs := NewCacheStores(logger)
	// // assert that the cacheStores map is not nil
	// assert.NotNil(t, cs.cs)

	d := NewDriver(logger)

	i1 := NewTestIngressV1("test1", "test")
	i2 := NewTestIngressV1("test2", "test")
	i3 := NewTestIngressV1("test4", "other")
	d.Add(i1)
	d.Add(i2)
	d.Add(i3)
	ings := d.Store.ListIngressesV1()
	if len(ings) != 3 {
		t.Errorf("expected 3 ingresses, got %d", len(ings))
	}

	for _, ing := range ings {
		fmt.Printf("Found ing: %v \n", ing)
	}

	i1CP, err := d.Store.GetIngressV1("test1", "test")
	if err != nil {
		fmt.Printf("err: %v", err)
		t.Errorf("expected ingress to be found")
		return
	}
	if i1CP == nil {
		t.Errorf("expected ingress to be found")
		return
	}
	if i1CP.Name != "test1" {
		t.Errorf("expected ingress name to be test1, got %s", i1CP.Name)
	}
	i2CP, err := d.Store.GetIngressV1("test2", "test")
	if err != nil {
		t.Errorf("expected ingress to be found")
		return
	}
	if i2CP == nil {
		t.Errorf("expected ingress to be found")
		return
	}
	if i2CP.Name != "test2" {
		t.Errorf("expected ingress name to be test2, got %s", i1CP.Name)
	}
}

func NewTestIngressV1(name string, namespace string) *netv1.Ingress {
	return &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									Path: "/",
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: "example",
											Port: netv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
