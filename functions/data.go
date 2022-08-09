package functions

//dataFunction (value, expectedValue, checkType)
type dataFunction = func(any, any, bool) error

var strDataFuncToFunc = map[string]dataFunction{
	"isEmpty": isEmpty,
}

//func typeOf(data any) string {
//	switch dt := data.(type) {
//		case
//	}
//}

func isEmpty(val any, _ any, checkType bool) error {
	return nil
}
