package store

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	ingressv1alpha1 "github.com/ngrok/kubernetes-ingress-controller/api/v1alpha1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	// . "github.com/onsi/ginkgo/v2"
	// . "github.com/onsi/gomega"
)

// var _ = Describe("Books", func() {

// })

func TestDriver(t *testing.T) {
	// // create a fake logger to pass into the cachestore
	logger := logr.New(logr.Discard().GetSink())
	// // create a new CacheStores object
	// cs := NewCacheStores(logger)
	// // assert that the cacheStores map is not nil
	// assert.NotNil(t, cs.cs)

	d := NewDriver(logger, runtime.NewScheme())

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

	foundIngs := d.ListIngressesV1()
	if len(foundIngs) != 6 {
		t.Errorf("expected 6 ingresses, got %d", len(foundIngs))
	}

	i1CP, err := d.GetIngressV1("test1", "test")
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
	i2CP, err := d.GetIngressV1("test2", "test")
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

	d := NewDriver(logger, runtime.NewScheme())
	d.Update(&icUsNotDefault)
	d.Update(&iMatching)
	d.Update(&iNotMatching)
	d.Update(&iNoClass)

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var d = NewDriver(logger, runtime.NewScheme())
			for _, ic := range scenario.ingressClasses {
				err := d.Update(&ic)
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
			d.Update(&iMatching)
			d.Update(&iNotMatching)
			d.Update(&iNoClass)

			foundIngs := d.ListNgrokIngressesV1()
			if len(foundIngs) != scenario.expectedIngresses {
				ings := d.ListIngressesV1()
				ngrokIngs := d.ListNgrokIngressesV1()
				ingClasses := d.ListIngressClassesV1()
				ngrokIngClasses := d.ListNgrokIngressClassesV1()

				t.Errorf("Found: ings: %+v \n ngrokIngs: %+v \n ingClasses: %+v \n ngrokIngClasses: %+v", ings, ngrokIngs, ingClasses, ngrokIngClasses)
				// t.Errorf("expected %d ingresses, got %d \nThe store had these ingresses %+v\n", scenario.expectedIngresses, len(foundIngs), d.ListIngressesV1())
			}
		})
	}
}

func makeTestTunnel(namespace, serviceName string, servicePort int) ingressv1alpha1.Tunnel {
	return ingressv1alpha1.Tunnel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", serviceName, servicePort),
			Namespace: namespace,
		},
		Spec: ingressv1alpha1.TunnelSpec{
			ForwardsTo: fmt.Sprintf("%s.%s.%s:%d", serviceName, namespace, clusterDomain, servicePort),
			Labels: map[string]string{
				"k8s.ngrok.com/namespace": namespace,
				"k8s.ngrok.com/service":   serviceName,
				"k8s.ngrok.com/port":      fmt.Sprintf("%d", servicePort),
			},
		},
	}
}

// func TestIngressReconcilerIngressToEdge(t *testing.T) {
// 	prefix := netv1.PathTypePrefix
// 	testCases := []struct {
// 		testName string
// 		ingress  *netv1.Ingress
// 		edge     *ingressv1alpha1.HTTPSEdge
// 		err      error
// 	}{
// 		{
// 			testName: "Returns a nil edge when ingress is nil",
// 			ingress:  nil,
// 			edge:     nil,
// 		},
// 		{
// 			testName: "Returns a nil edge when ingress has no rules",
// 			ingress: &netv1.Ingress{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name: "test-ingress",
// 				},
// 				Spec: netv1.IngressSpec{
// 					Rules: []netv1.IngressRule{},
// 				},
// 			},
// 			edge: nil,
// 		},
// 		{
// 			testName: "",
// 			ingress: &netv1.Ingress{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "test-ingress",
// 					Namespace: "test-namespace",
// 					Annotations: map[string]string{
// 						"k8s.ngrok.com/https-compression": "false",
// 					},
// 				},
// 				Spec: netv1.IngressSpec{
// 					Rules: []netv1.IngressRule{
// 						{
// 							Host: "my-test-tunnel.ngrok.io",
// 							IngressRuleValue: netv1.IngressRuleValue{
// 								HTTP: &netv1.HTTPIngressRuleValue{
// 									Paths: []netv1.HTTPIngressPath{
// 										{
// 											Path:     "/",
// 											PathType: &prefix,
// 											Backend:  makeTestBackend("test-service", 8080),
// 										},
// 									},
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 			edge: &ingressv1alpha1.HTTPSEdge{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "test-ingress",
// 					Namespace: "test-namespace",
// 				},
// 				Spec: ingressv1alpha1.HTTPSEdgeSpec{
// 					Hostports: []string{"my-test-tunnel.ngrok.io:443"},
// 					Routes: []ingressv1alpha1.HTTPSEdgeRouteSpec{
// 						{
// 							Match:     "/",
// 							MatchType: "path_prefix",
// 							Backend: ingressv1alpha1.TunnelGroupBackend{
// 								Labels: map[string]string{
// 									"k8s.ngrok.com/namespace": "test-namespace",
// 									"k8s.ngrok.com/service":   "test-service",
// 									"k8s.ngrok.com/port":      "8080",
// 								},
// 							},
// 							Compression: &ingressv1alpha1.EndpointCompression{
// 								Enabled: false,
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			testName: "",
// 			ingress: &netv1.Ingress{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "test-ingress",
// 					Namespace: "test-namespace",
// 					Annotations: map[string]string{
// 						"k8s.ngrok.com/https-compression": "true",
// 						"k8s.ngrok.com/ip-policy-ids":     "policy-1,policy-2",
// 						"k8s.ngrok.com/tls-min-version":   "1.3",
// 					},
// 				},
// 				Spec: netv1.IngressSpec{
// 					Rules: []netv1.IngressRule{
// 						{
// 							Host: "my-test-tunnel.ngrok.io",
// 							IngressRuleValue: netv1.IngressRuleValue{
// 								HTTP: &netv1.HTTPIngressRuleValue{
// 									Paths: []netv1.HTTPIngressPath{
// 										{
// 											Path:     "/",
// 											PathType: &prefix,
// 											Backend:  makeTestBackend("test-service", 8080),
// 										},
// 									},
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 			edge: &ingressv1alpha1.HTTPSEdge{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "test-ingress",
// 					Namespace: "test-namespace",
// 				},
// 				Spec: ingressv1alpha1.HTTPSEdgeSpec{
// 					Hostports: []string{"my-test-tunnel.ngrok.io:443"},
// 					TLSTermination: &ingressv1alpha1.EndpointTLSTerminationAtEdge{
// 						MinVersion: "1.3",
// 					},
// 					Routes: []ingressv1alpha1.HTTPSEdgeRouteSpec{
// 						{
// 							Match:     "/",
// 							MatchType: "path_prefix",
// 							Backend: ingressv1alpha1.TunnelGroupBackend{
// 								Labels: map[string]string{
// 									"k8s.ngrok.com/namespace": "test-namespace",
// 									"k8s.ngrok.com/service":   "test-service",
// 									"k8s.ngrok.com/port":      "8080",
// 								},
// 							},
// 							Compression: &ingressv1alpha1.EndpointCompression{
// 								Enabled: true,
// 							},
// 							IPRestriction: &ingressv1alpha1.EndpointIPPolicy{
// 								IPPolicyIDs: []string{"policy-1", "policy-2"},
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}

// 	for _, testCase := range testCases {
// 		irec := IngressReconciler{
// 			AnnotationsExtractor: annotations.NewAnnotationsExtractor(),
// 		}
// 		edge, err := irec.ingressToEdge(context.Background(), testCase.ingress)

// 		if testCase.err != nil {
// 			assert.ErrorIs(t, err, testCase.err)
// 			continue
// 		}
// 		assert.NoError(t, err)

// 		if testCase.edge == nil {
// 			assert.Nil(t, edge)
// 			continue
// 		}

// 		assert.Equal(t, testCase.edge, edge, "Edge does not match expected value")
// 	}
// }

// func TestIngressToTunnels(t *testing.T) {

// 	testCases := []struct {
// 		testName string
// 		ingress  *netv1.Ingress
// 		tunnels  []ingressv1alpha1.Tunnel
// 	}{
// 		{
// 			testName: "Returns empty list when ingress is nil",
// 			ingress:  nil,
// 			tunnels:  []ingressv1alpha1.Tunnel{},
// 		},
// 		{
// 			testName: "Returns empty list when ingress has no rules",
// 			ingress: &netv1.Ingress{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "test-ingress",
// 					Namespace: "test-namespace",
// 				},
// 				Spec: netv1.IngressSpec{
// 					Rules: []netv1.IngressRule{},
// 				},
// 			},
// 			tunnels: []ingressv1alpha1.Tunnel{},
// 		},
// 		{
// 			testName: "Converts an ingress to a tunnel",
// 			ingress: &netv1.Ingress{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "test-ingress",
// 					Namespace: "test-namespace",
// 				},
// 				Spec: netv1.IngressSpec{
// 					Rules: []netv1.IngressRule{
// 						{
// 							Host: "my-test-tunnel.ngrok.io",
// 							IngressRuleValue: netv1.IngressRuleValue{
// 								HTTP: &netv1.HTTPIngressRuleValue{
// 									Paths: []netv1.HTTPIngressPath{
// 										{
// 											Path:    "/",
// 											Backend: makeTestBackend("test-service", 8080),
// 										},
// 									},
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 			tunnels: []ingressv1alpha1.Tunnel{
// 				makeTestTunnel("test-namespace", "test-service", 8080),
// 			},
// 		},
// 		{
// 			testName: "Correctly converts an ingress with multiple paths that point to the same service",
// 			ingress: &netv1.Ingress{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "test-ingress",
// 					Namespace: "test-namespace",
// 				},
// 				Spec: netv1.IngressSpec{
// 					Rules: []netv1.IngressRule{
// 						{
// 							Host: "my-test-tunnel.ngrok.io",
// 							IngressRuleValue: netv1.IngressRuleValue{
// 								HTTP: &netv1.HTTPIngressRuleValue{
// 									Paths: []netv1.HTTPIngressPath{
// 										{
// 											Path:    "/",
// 											Backend: makeTestBackend("test-service", 8080),
// 										},
// 										{
// 											Path:    "/api",
// 											Backend: makeTestBackend("test-api", 80),
// 										},
// 									},
// 								},
// 							},
// 						},
// 						{
// 							Host: "my-other-test-tunnel.ngrok.io",
// 							IngressRuleValue: netv1.IngressRuleValue{
// 								HTTP: &netv1.HTTPIngressRuleValue{
// 									Paths: []netv1.HTTPIngressPath{
// 										{
// 											Path:    "/",
// 											Backend: makeTestBackend("test-service", 8080),
// 										},
// 										{
// 											Path:    "/api",
// 											Backend: makeTestBackend("test-api", 80),
// 										},
// 									},
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 			tunnels: []ingressv1alpha1.Tunnel{
// 				makeTestTunnel("test-namespace", "test-service", 8080),
// 				makeTestTunnel("test-namespace", "test-api", 80),
// 			},
// 		},
// 	}

// 	for _, test := range testCases {
// 		tunnels := ingressToTunnels(test.ingress)
// 		assert.ElementsMatch(t, tunnels, test.tunnels)
// 	}
// }

func makeTestBackend(serviceName string, servicePort int32) netv1.IngressBackend {
	return netv1.IngressBackend{
		Service: &netv1.IngressServiceBackend{
			Name: serviceName,
			Port: netv1.ServiceBackendPort{
				Number: servicePort,
			},
		},
	}
}
