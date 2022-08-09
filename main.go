// nolint: govet, golint
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/alecthomas/repr"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Trpc struct {
	Pos lexer.Position

	Entries []*Entry `( @@ ";"* )*`
}

type Entry struct {
	Pos lexer.Position

	TestName       string    `  "test" @String`
	Description    string    `  "desc" @String`
	TrpcVersion    string    `  "trpc" @String`
	MaxTime        float64   `  ("maxtime" @Float)?`
	Timeout        float64   `  ("timeout" @Float)?`
	VerboseLevel   int       `  ("verbose" @Int)?`
	ImportPath     string    `| "importpath" @String`
	ImportProto    string    `| "import" "protofile" @String`
	ImportProtoSet string    `| "import" "protoset" @String`
	Endpoint       *Endpoint `| @@`
	Invoke         *Invoke   `| @@`

	Lines    *[]string
	Warnings int
	Ignores  int
}

type Endpoint struct {
	Pos lexer.Position

	Name     string `"endpoint" @Ident`
	Tls      bool   `(@"tls")?`
	IPDomain string `@String`
	//IPDomain             string `( @(Ident ( "." Ident )*) | @([0-9]{1,3} ( "." [0-9]{1,3})*) )`
	Port                 int    `"port" @Int`
	PerfixPath           string `("path" @("/" Ident ( "/" Ident )*))?`
	ReflectionPerfixPath string `("reflectPath" @("/" Ident ( "/" Ident )*))?`
	IgnoreTrailers       bool   `(@"ignTrailer")?`
}

type Invoke struct {
	Pos lexer.Position

	Name          string    `"invoke" @Ident`
	EndPoint      string    `@Ident`
	Service       string    `@Ident @( "." Ident )*`
	RPC           string    `@Ident`
	Goal          string    `( "goal" @String )?`
	Headers       []*Header `("headers" "{" @@* "}")?`
	Data          []*Data   `("data" "{" @@* "}")?`
	SourceExpects []*Expect `("expects" "{" @@* "}")?`
	// These ones are runtime extracted values
	Entry              *Entry
	ContaineReferences bool
	RequestData        map[string]any
	RequestHeaders     []string
	Expects            []Expect
	Response           *map[string]any
	ResponseJson       string
	ResponseHeaders    []string
	Conditions         []InvokeCondition
}

type NamedInvokes = map[string]*Invoke

type InvokeConditionStatus = int

const (
	InvokeDoneWithIgnores InvokeConditionStatus = iota
	InvokeDoneWithWarnings
	InvokeFailed
)

type InvokeCondition struct {
	Condition InvokeConditionStatus
	Expect    *Expect
	Msg       string
}

func (i InvokeCondition) String() string {
	switch i.Condition {
	case InvokeDoneWithWarnings:
		return "Warn"
	case InvokeDoneWithIgnores:
		return "Ignore"
	case InvokeFailed:
		return "Panic"
	default:
		panic("Invalid invoke status")
	}
}

func NewInvokeCondition(conditionName string, expect *Expect, msg string) InvokeCondition {
	var condition = InvokeFailed
	switch conditionName {
	case "Ignore":
		condition = InvokeDoneWithIgnores
	case "Warn":
		condition = InvokeDoneWithWarnings
	case "Panic":
		condition = InvokeFailed
	}

	invokeCondition := InvokeCondition{
		Condition: condition,
		Expect:    expect,
		Msg:       msg,
	}

	expect.Invoke.Conditions = append(expect.Invoke.Conditions, invokeCondition)

	return invokeCondition
}

type Data struct {
	Pos lexer.Position

	Key   string `(@Ident ":"`
	Value Value  ` @@ (",")?) `
}

type Value struct {
	Pos lexer.Position

	String    *string   `  @String`
	Reference *PathExpr `| @@`
	RawString *string   `| @Ident`
	Float     *float64  `| @Float`
	Int       *int64    `| @Int`
	Bool      *bool     `| (@"true" | "false")`
	Map       *Map      `| @@`
	Array     *Array    `| @@`
}

type PathExpr struct {
	Parts []Part `@@ ( "." @@ )*`
}

func (p PathExpr) String() string {
	parts := []string{}
	for _, part := range p.Parts {
		parts = append(parts, part.String())
	}
	return strings.Join(parts, ".")
}

type Part struct {
	Obj string `@Ident`
	Acc []Acc  `("[" @@ "]")*`
	//Param []Value `| @("(" @@ ")")`
}

func (p Part) String() string {
	str := p.Obj
	if len(p.Acc) > 0 {
		for _, acc := range p.Acc {
			str = str + fmt.Sprintf("[%s]", acc.String())
		}
	}
	return str
}

type Acc struct {
	StrIndex *string `@(String|Char|RawString)`
	IntIndex *int    `| @Int`
}

func (a Acc) String() string {
	if a.StrIndex != nil {
		return *a.StrIndex
	} else {
		return fmt.Sprintf("%d", *a.IntIndex)
	}
}

type Header struct {
	Pos lexer.Position

	Key   string `(@String ":"`
	Value string ` @String )(",")? `
}

type Array struct {
	Pos lexer.Position

	Elements []*Value `"[" ( @@ ( ","? @@ )* )? "]"`
}

type Map struct {
	Pos lexer.Position

	Entries []*MapEntry `"{" ( @@ ( ( "," )? @@ )* )? "}"`
}

type MapEntry struct {
	Pos lexer.Position

	Key   string `( @Ident`
	Value Value  `":" @@)`
}

type Expect struct {
	Pos lexer.Position

	Path     *PathExpr `@@`
	Function *Function `@@`
	OnFail   *string   `( "onFail" @("Panic"|"Warn"|"Ignore") )?`
	//Code      []string
	Invoke *Invoke
}

type Function struct {
	Pos lexer.Position

	Name string `( @Ident`
	Arg  Value  ` "(" (@@)? ")" )`
}

var (
	parser = participle.MustBuild(&Trpc{}, participle.UseLookahead(2))
	cli    struct {
		Files []string `required existing file arg help:"TRPC(Test RPC) file(s).\n trpc file or trpc file1 file2 or trpc *.trpc"`
	}
)

func (value Value) value(lines *[]string, namedInvokes *NamedInvokes, withReference bool) (interface{}, bool) {
	var haveReference bool = false
	if value.Map != nil {
		result := make(map[string]interface{})
		for _, entry := range value.Map.Entries {
			result[entry.Key], haveReference = entry.Value.value(lines, namedInvokes, withReference)
		}
		return result, haveReference
	} else if value.Array != nil {
		result := make([]interface{}, len(value.Array.Elements))
		for i, element := range value.Array.Elements {
			result[i], haveReference = element.value(lines, namedInvokes, withReference)
		}
		return result, haveReference
	} else if value.String != nil {
		val, _ := strconv.Unquote(*value.String)
		return val, haveReference
	} else if value.RawString != nil {
		return *value.RawString, false
	} else if value.Float != nil {
		return *value.Float, haveReference
	} else if value.Int != nil {
		return *value.Int, haveReference
	} else if value.Bool != nil {
		return *value.Bool, haveReference
	} else if value.Reference != nil {
		parts := value.Reference.Parts
		if parts[1].Obj != "response" && parts[1].Obj != "data" {
			syntaxError(lines, value.Pos, 0, "Invalid reference: %s", value.Reference)
		}
		if invoke, ok := (*namedInvokes)[parts[0].Obj]; ok {
			if withReference {
				if invoke.Response == nil {
					syntaxError(lines, value.Pos, 0, "Reference error %v, invoke must be called before use!", value.Reference)
					return nil, false
				}
				var reference map[string]any
				if parts[1].Obj == "response" {
					reference = (*invoke.Response)
				} else {
					reference = invoke.RequestData
				}
				parts = parts[2:]
				for {
					if len(parts) == 1 {
						//fmt.Printf("ref 1 part %s : %v\n", parts[0].Obj, response[parts[0].Obj])
						return reference[parts[0].Obj], true
					}
					//fmt.Printf("ref 2 %v -> %v\n", parts[0].Obj, parts[0].Acc)
					if len(parts[0].Acc) == 0 {
						reference = reference[parts[0].Obj].(map[string]interface{})
					} else {
						if parts[0].Acc[0].StrIndex != nil {
							reference = reference[parts[0].Obj].(map[string]interface{})[*parts[0].Acc[0].StrIndex].(map[string]interface{})
						} else {
							reference = reference[parts[0].Obj].(map[string]interface{})[strconv.Itoa(*parts[0].Acc[0].IntIndex)].(map[string]interface{})
						}
					}
					parts = parts[1:]
				}

			}
		} else {
			syntaxError(lines, value.Pos, 0, "Reference not found %v", value.Reference)
			return nil, false
		}
		return value.Reference, true
	}
	return nil, false
}

func (invoke *Invoke) Parse(lines *[]string, namedInvokes *NamedInvokes, withReference bool, parseHeaders bool) error {
	if invoke.Data == nil {
		return nil
	}
	invoke.RequestData = make(map[string]interface{})
	haveReference := false
	for _, data := range invoke.Data {
		var valueHaveReference bool
		invoke.RequestData[data.Key], valueHaveReference = data.Value.value(lines, namedInvokes, withReference)
		haveReference = haveReference || valueHaveReference
	}
	invoke.ContaineReferences = haveReference
	//parsing Headers must run once on the parsing file
	if parseHeaders && invoke.Headers != nil {
		invoke.RequestHeaders = make([]string, len(invoke.Headers))
		for i, header := range invoke.Headers {
			key, _ := strconv.Unquote(header.Key)
			value, _ := strconv.Unquote(header.Value)
			invoke.RequestHeaders[i] = fmt.Sprintf("%s=%s", key, value)
		}
	}
	return nil
}

func (invoke *Invoke) ParsedDataWithReference(namedInvokes *map[string]Invoke) {

}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func main() {
	ctx := kong.Parse(&cli)

	for _, file := range cli.Files {
		trpc := &Trpc{}
		trpcFile, err := os.Open(file)
		lines, _ := readLines(file)
		ctx.FatalIfErrorf(err, "")
		err = parser.Parse("", trpcFile, trpc)
		ctx.FatalIfErrorf(err, "")
		if false {
			repr.Println(trpc /*, repr.Hide(&lexer.Position{})*/)
		}
		if len(trpc.Entries) > 0 {
			trpc.Entries[0].Lines = &lines
			TasteAndRun(trpc)
		}
	}
}

/*

replace github.com/jhump/protoreflect => github.com/nima-dvlp/protoreflect v1.12.1

replace github.com/fullstorydev/grpcurl => github.com/nima-dvlp/grpcurl v1.8.8
*/
