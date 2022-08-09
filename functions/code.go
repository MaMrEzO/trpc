package functions

import (
	"fmt"

	"google.golang.org/grpc/codes"
)

type codeFunction = func(codes.Code) error

var strCodeFromFnName = map[string]codes.Code{
	"isOk":                 codes.OK,
	"isCanceled":           codes.Canceled,
	"isUnknown":            codes.Unknown,
	"isInvalidArgument":    codes.InvalidArgument,
	"isDeadlineExceeded":   codes.DeadlineExceeded,
	"isNotFound":           codes.NotFound,
	"isAlreadyExists":      codes.AlreadyExists,
	"isPermissionDenied":   codes.PermissionDenied,
	"isResourceExhausted":  codes.ResourceExhausted,
	"isFailedPrecondition": codes.FailedPrecondition,
	"isAborted":            codes.Aborted,
	"isOutOfRange":         codes.OutOfRange,
	"isUnimplemented":      codes.Unimplemented,
	"isInternal":           codes.Internal,
	"isUnavailable":        codes.Unavailable,
	"isDataLoss":           codes.DataLoss,
	"isUnauthenticated":    codes.Unauthenticated,
}

func CodeFunction(fn string) (codeFunction, error) {
	if code, ok := strCodeFromFnName[fn]; ok {
		fn := func(expected codes.Code) error {
			if code == expected {
				return nil
			}
			return fmt.Errorf("Response code expected to be (%v) but got (%v)", expected, code)
		}
		return fn, nil
	}
	return nil, fmt.Errorf("Invalid function for examining code: \"%s\"", fn)
}
