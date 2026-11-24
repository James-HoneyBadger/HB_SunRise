// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

// Package errs is a slim stand-in for honey-ledger's pkg/errs. It keeps the
// Error type, the normal-error category and the helpers the money file parser
// uses, and drops everything the standalone programs do not need.
package errs

// ErrorCategory represents error category
type ErrorCategory int32

// Error categories
const (
	CATEGORY_SYSTEM ErrorCategory = 1
	CATEGORY_NORMAL ErrorCategory = 2
)

// Sub categories of normal error
const (
	NormalSubcategoryTransaction = 5
)

// Error represents the specific error returned to user
type Error struct {
	Category       ErrorCategory
	SubCategory    int32
	Index          int32
	HttpStatusCode int
	Message        string
	BaseError      []error
	Context        any
}

// Error returns the error message
func (err *Error) Error() string {
	return err.Message
}

// Code returns the error code
func (err *Error) Code() int32 {
	return int32(err.Category)*100000 + err.SubCategory*1000 + err.Index
}

// New returns a new error instance
func New(category ErrorCategory, subCategory int32, index int32, httpStatusCode int, message string, baseError ...error) *Error {
	return &Error{
		Category:       category,
		SubCategory:    subCategory,
		Index:          index,
		HttpStatusCode: httpStatusCode,
		Message:        message,
		BaseError:      baseError,
	}
}

// NewNormalError returns a new normal error instance
func NewNormalError(subCategory int32, index int32, httpStatusCode int, message string) *Error {
	return New(CATEGORY_NORMAL, subCategory, index, httpStatusCode, message)
}

// Or returns the error from err parameter if it is an Error defined in this
// project, or the default error otherwise.
func Or(err error, defaultErr *Error) *Error {
	if finalError, ok := err.(*Error); ok {
		return finalError
	}

	return defaultErr
}
