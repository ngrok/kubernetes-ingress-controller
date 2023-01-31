package errors

import (
	"fmt"
	"strings"

	netv1 "k8s.io/api/networking/v1"
)

// Not all domains are reconciled yet and have a domain in their status
type NotAllDomainsReadyYetError struct{}

// Error returns the error message
func (e *NotAllDomainsReadyYetError) Error() string {
	return "not all domains ready yet"
}

// NewNotAllDomainsReadyYetError returns a new NotAllDomainsReadyYetError
func NewNotAllDomainsReadyYetError() error {
	return &NotAllDomainsReadyYetError{}
}

// IsNotAllDomainsReadyYet returns true if the error is a NotAllDomainsReadyYetError
func IsNotAllDomainsReadyYet(err error) bool {
	_, ok := err.(*NotAllDomainsReadyYetError)
	return ok
}

type ErrNotFoundInStore struct {
	message string
}

func NewErrorNotFound(message string) ErrNotFoundInStore {
	return ErrNotFoundInStore{message: message}
}

func (e ErrNotFoundInStore) Error() string {
	return e.message
}

func IsErrorNotFound(err error) bool {
	_, ok := err.(ErrNotFoundInStore)
	return ok
}

type ErrDifferentIngressClass struct {
	message string
}

func NewErrDifferentIngressClass(ourIngressClasses []*netv1.IngressClass, foundIngressClass *string) ErrDifferentIngressClass {
	msg := []string{"The ingress object is not valid for this controllers ingress class configuration."}
	if foundIngressClass == nil {
		msg = append(msg, "The ingress object does not have an ingress class set.")
	} else {
		msg = append(msg, fmt.Sprintf("The ingress object has an ingress class set to %s.", *foundIngressClass))
	}
	for _, ingressClass := range ourIngressClasses {
		if ingressClass.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true" {
			msg = append(msg, fmt.Sprintf("This controller is the default ingress controller ingress class %s.", ingressClass.Name))
		}
		msg = append(msg, fmt.Sprintf("This controller is watching for the class %s", ingressClass.Name))
	}
	return ErrDifferentIngressClass{message: strings.Join(msg, "\n")}
}

func (e ErrDifferentIngressClass) Error() string {
	if e.message == "" {
		return "different ingress class"
	}
	return e.message
}

func IsErrDifferentIngressClass(err error) bool {
	_, ok := err.(ErrDifferentIngressClass)
	return ok
}

type ErrInvalidIngressSpec struct {
	errors []string
}

func NewErrInvalidIngressSpec() ErrInvalidIngressSpec {
	return ErrInvalidIngressSpec{}
}

func (e ErrInvalidIngressSpec) AddError(err string) {
	e.errors = append(e.errors, err)
}

func (e ErrInvalidIngressSpec) HasErrors() bool {
	return len(e.errors) > 0
}

func (e ErrInvalidIngressSpec) Error() string {
	return fmt.Sprintf("invalid ingress spec: %s", e.errors)
}

func IsErrInvalidIngressSpec(err error) bool {
	_, ok := err.(ErrInvalidIngressSpec)
	return ok
}
