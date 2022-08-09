package functions

import "fmt"

// messageFunc(value, expectedValue)
type messageFunc = func(string, string) error

var strMessageFuncToFunc = map[string]messageFunc{
	"isEqual":    msgIsEqual,
	"isEmpty":    msgIsEmpty,
	"isNotEmpty": msgIsNotEmpty,
	"hasValue":   msgIsNotEmpty,
}

const errHeader = "Response message "

func msgIsEqual(val string, expected string) error {
	if expected == val {
		return nil
	}
	return fmt.Errorf(errHeader+"expected to be \"%s\" but got \"%s\"", expected, val)
}

func msgIsEmpty(val string, _ string) error {
	if len(val) == 0 {
		return nil
	}
	return fmt.Errorf(errHeader+"expected to be empty but got \"%s\"", val)
}

func msgIsNotEmpty(val string, _ string) error {
	if len(val) > 0 {
		return nil
	}
	return fmt.Errorf(errHeader + "expected to have value but it is empty")
}

func MessageFunction(fn string) (messageFunc, error) {
	if fn, ok := strMessageFuncToFunc[fn]; ok {
		return fn, nil
	}
	return nil, fmt.Errorf("Invalid function for examining message: \"%s\"", fn)
}
