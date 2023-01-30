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

	names := []string{"test1", "test2", "test3"}
	namespaces := []string{"test", "other"}

	ings := []netv1.Ingress{}
	for _, name := range names {
		for _, namespace := range namespaces {
			ing := NewTestIngressV1(name, namespace)
			ings = append(ings, ing)
		}
	}
	for _, ing := range ings {
		err := d.Update(&ing)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	}

	foundIngs := d.Store.ListIngressesV1()
	if len(foundIngs) != 6 {
		t.Errorf("expected 6 ingresses, got %d", len(foundIngs))
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

func TestIngressClass(t *testing.T) {
	logger := logr.New(logr.Discard().GetSink())
	iMatching := NewTestIngressV1WithClass("test1", "test", "ngrok")
	iNotMatching := NewTestIngressV1WithClass("test2", "test", "test")
	iNoClass := NewTestIngressV1("test3", "test")
	icUsDefault := NewTestIngressClass("ngrok", true, true)
	icUsNotDefault := NewTestIngressClass("ngrok", false, true)
	icOtherDefault := NewTestIngressClass("test", true, false)
	icOtherNotDefault := NewTestIngressClass("test", false, false)

	// Ingress Class Scenarios
	// No classes
	// just us not as default
	// just us as default
	// just another not as default
	// just another as default
	// us and another neither default
	// us and another them default
	// us and another us default
	// us and another both default ?

	scenarios := []struct {
		name              string
		ingressClasses    []netv1.IngressClass
		ingresses         []netv1.Ingress
		expectedIngresses int
	}{
		{
			name:              "no ingress classes",
			ingressClasses:    []netv1.IngressClass{},
			expectedIngresses: 0,
		},
		{
			name:              "just us not as default",
			ingressClasses:    []netv1.IngressClass{icUsNotDefault},
			expectedIngresses: 1,
		},
		{
			name:              "just us as default",
			ingressClasses:    []netv1.IngressClass{icUsDefault},
			expectedIngresses: 2,
		},
		{
			name:              "just another not as default",
			ingressClasses:    []netv1.IngressClass{icOtherNotDefault},
			expectedIngresses: 0,
		},
		{
			name:              "just another as default",
			ingressClasses:    []netv1.IngressClass{icOtherDefault},
			expectedIngresses: 0,
		},
		{
			name:              "us and another neither default",
			ingressClasses:    []netv1.IngressClass{icUsNotDefault, icOtherNotDefault},
			expectedIngresses: 1,
		},
		{
			name:              "us and another them default",
			ingressClasses:    []netv1.IngressClass{icUsNotDefault, icOtherDefault},
			expectedIngresses: 1,
		},
		{
			name:              "us and another us default",
			ingressClasses:    []netv1.IngressClass{icUsDefault, icOtherNotDefault},
			expectedIngresses: 2,
		},
		{
			name:              "us and another both default",
			ingressClasses:    []netv1.IngressClass{icUsDefault, icOtherDefault},
			expectedIngresses: 2,
		},
	}

	d := NewDriver(logger)
	d.Update(&icUsNotDefault)
	d.Update(&iMatching)
	d.Update(&iNotMatching)
	d.Update(&iNoClass)

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var d = NewDriver(logger)
			for _, ic := range scenario.ingressClasses {
				err := d.Update(&ic)
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
			d.Update(&iMatching)
			d.Update(&iNotMatching)
			d.Update(&iNoClass)

			foundIngs := d.Store.ListNgrokIngressesV1()
			if len(foundIngs) != scenario.expectedIngresses {
				ings := d.Store.ListIngressesV1()
				ngrokIngs := d.Store.ListNgrokIngressesV1()
				ingClasses := d.Store.ListIngressClassesV1()
				ngrokIngClasses := d.Store.ListNgrokIngressClassesV1()

				t.Errorf("Found: ings: %+v \n ngrokIngs: %+v \n ingClasses: %+v \n ngrokIngClasses: %+v", ings, ngrokIngs, ingClasses, ngrokIngClasses)
				// t.Errorf("expected %d ingresses, got %d \nThe store had these ingresses %+v\n", scenario.expectedIngresses, len(foundIngs), d.Store.ListIngressesV1())
			}
		})
	}
}

func NewTestIngressClass(name string, isDefault bool, isNgrok bool) netv1.IngressClass {
	i := netv1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/component": "controller",
			},
		},
	}

	if isNgrok {
		i.Spec.Controller = controllerName
	} else {
		i.Spec.Controller = "kubernetes.io/ingress-other"
	}

	if isDefault {
		i.Annotations = map[string]string{
			"ingressclass.kubernetes.io/is-default-class": "true",
		}
	}

	return i
}

func NewTestIngressV1WithClass(name string, namespace string, ingressClass string) netv1.Ingress {
	i := NewTestIngressV1(name, namespace)
	i.Spec.IngressClassName = &ingressClass
	return i
}

func NewTestIngressV1(name string, namespace string) netv1.Ingress {
	return netv1.Ingress{
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
