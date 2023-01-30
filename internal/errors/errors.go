package errors

import "fmt"

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

func NewErrDifferentIngressClass(ourIngressClasses []string, foundIngressClass string) ErrDifferentIngressClass {
	return ErrDifferentIngressClass{message: fmt.Sprintf("different ingress class, our ingress classes: %s, found ingress class: %s", ourIngressClasses, foundIngressClass)}
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
