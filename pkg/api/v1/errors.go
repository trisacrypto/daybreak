package api

import (
	"fmt"

	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/x/validation"
)

//=============================================================================
// Error Replies
//=============================================================================

var (
	NotFound      = Reply{Success: false, Error: "resource not found"}
	NotAllowed    = Reply{Success: false, Error: "method not allowed"}
	InternalError = Reply{Success: false, Error: "an internal error occurred"}
)

// Error constructs a new reply for an error value.
func Error(status int, err error, msg string) *errors.HTTP {
	if err == nil {
		return &errors.HTTP{
			Status: status,
			Err:    errors.ErrUnknown,
			Reply:  Reply{Success: false, Error: errors.ErrUnknown.Error()},
		}
	}

	if msg == "" {
		msg = err.Error()
	}

	herr := &errors.HTTP{
		Status: status,
		Err:    err,
		Reply:  Reply{Success: false, Error: msg},
	}

	if verr, ok := err.(validation.Errors); ok {
		herr.Reply = validationReply(verr)
	}

	return herr
}

//=============================================================================
// Error Detail
//=============================================================================

// ErrorDetail is a list of per-field validation errors.
type ErrorDetail []*DetailError

// DetailError describes a specific invalid field in a request payload.
type DetailError struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

//=============================================================================
// Helpers
//=============================================================================

// validationReply converts validation errors to a structured error reply.
func validationReply(errs validation.Errors) Reply {
	rep := Reply{Success: false}
	if len(errs) == 1 {
		rep.Error = errs.Error()
		return rep
	}

	rep.Error = fmt.Sprintf("%d validation errors occurred", len(errs))
	rep.ErrorDetail = make(ErrorDetail, 0, len(errs))
	for _, verr := range errs {
		rep.ErrorDetail = append(rep.ErrorDetail, &DetailError{
			Field: verr.Field(),
			Error: verr.Error(),
		})
	}
	return rep
}
