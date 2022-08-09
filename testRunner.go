package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"

	"trpc/functions"
	"trpc/grpcrunner"
	"trpc/trpc_marshal"

	"github.com/alecthomas/participle/v2/lexer"
	//"bytes"
	//"github.com/fullstorydev/grpcurl"
	//"github.com/golang/protobuf/jsonpb"
	//"github.com/golang/protobuf/proto"
)

const acceptableVersion = "v0.0.1"

func TasteAndRun(trpc *Trpc) {
	verbose := 0
	_maxTime := 0.0
	maxTime := &_maxTime
	_timeOut := 0.0
	connectionTimeout := &_timeOut

	//keepalive := &_timeOut
	protoImportPaths := make([]string, 0)
	protoFiles := make([]string, 0)
	protoSets := make([]string, 0)
	namedEndpointes := make(map[string]Endpoint, 0)

	invokeOrder := make([]string, 0)
	namedInvokes := make(NamedInvokes, 0)

	defaultFailBehave := "Panic"

	//expects := make([]Expect, 0)

	namedInvokeHandlers := make(map[string]grpcrunner.TRPCHandler, 0)

	mainEntery := trpc.Entries[0]
	if len(mainEntery.TestName) == 0 {
		print("Be kind and name your test")
		return
	} else {
		if mainEntery.MaxTime > 0 {
			maxTime = &mainEntery.MaxTime
		}
		if mainEntery.Timeout > 0 {
			connectionTimeout = &mainEntery.Timeout
		}
		verbose = mainEntery.VerboseLevel
	}

	for _, entery := range trpc.Entries[1:] {
		if entery.Description != "" {
		}
		if len(entery.ImportPath) > 0 {
			iPath, _ := strconv.Unquote(entery.ImportPath)
			protoImportPaths = append(protoImportPaths, iPath)
		}
		if len(entery.ImportProto) > 0 {
			pFile, _ := strconv.Unquote(entery.ImportProto)
			protoFiles = append(protoFiles, pFile)
		}
		if len(entery.ImportProtoSet) > 0 {
			pSet, _ := strconv.Unquote(entery.ImportProtoSet)
			protoSets = append(protoSets, pSet)
		}

		if entery.Endpoint != nil {
			unQuoted, _ := strconv.Unquote(entery.Endpoint.IPDomain)
			entery.Endpoint.IPDomain = unQuoted
			namedEndpointes[entery.Endpoint.Name] = *entery.Endpoint
		}

		if entery.Invoke != nil {
			if existInvoke, exists := namedInvokes[entery.Invoke.Name]; exists {
				fmt.Printf("Duplicate invoke name %s in %s:%d which defined at %s:%d\n", entery.Invoke.Name, entery.Pos.Filename, entery.Pos.Line, existInvoke.Pos.Filename, existInvoke.Pos.Line)
				return
			} else {
				//jb, _ := json.MarshalIndent(entery.Invoke.ParsedData, "", "  ")
				//println(entery.Invoke.Name, "  ", string(jb), entery.Invoke.ParsedData, entery.Invoke.Headers)
				namedInvokes[entery.Invoke.Name] = entery.Invoke
				entery.Invoke.Parse((*mainEntery).Lines, &namedInvokes, false, true)
				entery.Invoke.Response = nil
				entery.Invoke.Goal, _ = strconv.Unquote(entery.Invoke.Goal)
				invokeOrder = append(invokeOrder, entery.Invoke.Name)
				entery.Invoke.Entry = mainEntery
				entery.Invoke.Conditions = make([]InvokeCondition, 0)
				for _, expect := range entery.Invoke.SourceExpects {
					//expect.Reference. = entery.Invoke.Name + "." + expect.Reference
					//expect.Code = strings.Split(expect.Reference, ".")
					expect.Invoke = entery.Invoke
					if expect.OnFail == nil {
						expect.OnFail = &defaultFailBehave
					} else if *expect.OnFail != "Warn" && *expect.OnFail != "Ignore" {
						syntaxError(mainEntery.Lines, expect.Pos, 0, "Invalid onFail param", *expect.OnFail)
					}
					entery.Invoke.Expects = append(entery.Invoke.Expects, *expect)
				}
			}
		}
	}

	for _, invokeName := range invokeOrder {
		invoke := namedInvokes[invokeName]

		fmt.Println("===========================\ninvoke: ", invokeName)
		if endPoint, ok := namedEndpointes[invoke.EndPoint]; ok {
			//fmt.Printf("call %s:%d%s/%s/%s %v\n", endPoint.IPDomain, endPoint.Port, endPoint.PerfixPath, invoke.Service, invoke.RPC, invoke.ContaineReferences)
			if invoke.ContaineReferences {
				invoke.Parse((*mainEntery).Lines, &namedInvokes, true, false)
			}

			handler, _ := grpcrunner.Run(grpcrunner.RunParams{
				ProtoFiles:  protoFiles,
				ImportPaths: protoImportPaths,
				Protoset:    protoSets,
				Target:      fmt.Sprintf("%s:%d", endPoint.IPDomain, endPoint.Port),
				PrefixPath:  endPoint.PerfixPath,
				// ReflectionPrefixPath: endPoint.ReflectionPerfixPath,
				Plaintext:      !endPoint.Tls,
				Data:           invoke.RequestData,
				RPCHeaders:     invoke.RequestHeaders,
				ServiceName:    invoke.Service,
				MethodName:     invoke.RPC,
				Verbose:        verbose <= 2,
				VeryVerbose:    verbose == 2,
				MaxTime:        *maxTime,
				KeepaliveTime:  0.0,
				ConnectTimeout: *connectionTimeout,
			})
			//if err != nil {
			//	if false {
			//		fmt.Println(err)
			//	}
			//}
			namedInvokeHandlers[invokeName] = *handler
			//var message protoiface.MessageV1
			if len(handler.ResponseData) > 0 {
				//message = handler.ResponseData[0]
				//response := make(map[string]interface{})

				//jsonB, jErr := marshal(message, handler.Descriptor)
				//if jErr != nil {
				//	println("error on marshal message: ", jErr)
				//} else {
				//	println("message json: ", string(jsonB))
				//}
				//jErr = json.Unmarshal(jsonB, &response)
				//invoke.ResponseJson = string(jsonB)
				//invoke.Response = &response
				//fmt.Printf("MD: %v\n------------------------------------\n", handler.MethodDescriptor)
				////mdReq := handler.MethodDescriptor.GetInputType()
				//mdRes := handler.MethodDescriptor.GetOutputType()
				////fmt.Printf("MdReq: %s: %v\n------------------------------------\n", mdReq.GetFullyQualifiedName(), mdReq)
				//fmt.Printf("MdRes: %s: %v\n------------------------------------\n", mdRes.GetFullyQualifiedName(), mdRes)
				////refl := proto.MessageReflect(message)
				//refl2, _ := dynamic.AsDynamicMessage(message)
				//for _, fld := range mdRes.GetFields() {
				//	if fld.IsRepeated() {
				//		repeated := refl2.GetField(fld).([]interface{})
				//		fmt.Printf("fld is repeated (%T) with len: %d ~= %d\n", refl2.GetField(fld), len(repeated), refl2.FieldLength(fld))
				//		for i, v := range repeated {
				//			fmt.Printf("fld(rp) %v[%d] =>(%T) %v\n", fld, i, v, v.(*dynamic.Message))
				//		}
				//	} else {
				//		fmt.Printf("fld => %v: %v\n", fld, refl2.GetField(fld))
				//	}
				//}
				m := trpc_marshal.RPCMessageToMap(*handler)
				//fmt.Printf("----------------------------------===\nmarshaled : %v\n", m)
				jb, _ := json.MarshalIndent(m, "", "  ")
				//fmt.Println("----------====------------\n" + string(jb))
				invoke.Response = &m
				invoke.ResponseJson = string(jb)

				//md, err := desc.LoadMessageDescriptorForMessage(message)
				//namedFields := make(map[string]*desc.FieldDescriptor)
				//var fields []*desc.FieldDescriptor
				//if err != nil {
				//	println("error: ", err)
				//} else {
				//	//println("message desc: ", md.String())
				//	fields = md.GetFields()
				//	for _, field := range fields {
				//		//println("field: ", field.String())
				//		namedFields[field.GetName()] = field
				//		if _, exists := response[field.GetName()]; !exists {
				//			response[field.GetName()] = nil
				//		}
				//	}
				//}
				//invoke.NamedFields = namedFields
			} else {
				invoke.Response = &map[string]interface{}{}
				invoke.ResponseJson = "{}"
			}

			for _, expect := range invoke.Expects {
				code := expect.Path.Parts

				if code[0].Obj == "code" {
					//fmt.Printf("code !? : <<%v>>\n", handler.Status)
					codeFn, err := functions.CodeFunction(expect.Function.Name)
					if err != nil {
						syntaxError((*mainEntery).Lines, invoke.Pos, 0, err.Error())
					}
					fnErr := codeFn(handler.Status.Code())
					if fnErr != nil {
						testFailed(mainEntery, invoke, &expect, 0, fnErr.Error())
					}
				} else if code[0].Obj == "message" {
					fn, err := functions.MessageFunction(expect.Function.Name)
					if err != nil {
						syntaxError((*mainEntery).Lines, invoke.Pos, 0, err.Error())
					}
					value, _ := expect.Function.Arg.value((*mainEntery).Lines, &namedInvokes, true)
					fnErr := fn(value.(string), handler.Status.Message())
					if fnErr != nil {
						testFailed(mainEntery, invoke, &expect, 0, fnErr.Error())
					}
				} else if code[0].Obj == "response" {
					if invoke.Response == nil {
						testFailed(mainEntery, invoke, &expect, 0, "Expect check value on invocation with error response")
					}

					if len(code) == 1 {
						if expect.Function.Name == "isEmpty" {
							if len(*invoke.Response) != 0 {
								testFailed(mainEntery, invoke, &expect, 0, "Response expected to be empty but it is not")
							}
						} else {
							syntaxError((*mainEntery).Lines, invoke.Pos, 0, "Unknown function %v", expect.Function.Name)
						}
					}
					code := code[1:]
					//fmt.Printf(" %s expect %v\n", invokeName, code)
					offset := len("expect " + invokeName + "." + "response.")
					if val, exists := (*invoke.Response)[code[0].Obj]; exists {
						parts := []Part{
							{
								Obj: invokeName,
							},
						}
						expect.Path.Parts = append(parts, expect.Path.Parts...)
						val, _ = Value{Reference: expect.Path}.value((*mainEntery).Lines, &namedInvokes, true)
						switch expect.Function.Name {
						case "hasValue":
							{
								if val == nil {
									println("Test failed: ", invokeName, " expected data is not null but got null")
								}
							}
						case "isNull":
							{
								if val != nil {
									testFailed(mainEntery, invoke, &expect, offset, "%s expected to be null but got (%v) %T\nActual response:\n%s", expect.Path.String(), val, val, invoke.ResponseJson)
								}
							}
						case "isEqual":
							{
								expectValue, _ := expect.Function.Arg.value((*mainEntery).Lines, &namedInvokes, true)
								if val != expectValue {
									testFailed(mainEntery, invoke, &expect, offset, "%s expected to be \"%v\" but got \"%v\"\n\nActual response:\n%s", expect.Path.String(), expectValue, val, invoke.ResponseJson)
								}
							}
						case "isNotEmpty":
							{
								if val == nil {
									println("Test failed: ", invokeName, " expected data is not empty but got null")
								} else if val == "" {
									println("Test failed: ", invokeName, " expected data is not empty but got empty string")
								}
							}
						default:
							fmt.Printf("Test failed unknown expect operator %s at %s:%d\n", expect.Function.Name, expect.Pos.Filename, expect.Pos.Line)
						}
					} else {
						testFailed(mainEntery, invoke, &expect, offset, "Field %s not found on %s", code[0], invoke.RPC)
					}

				} else {
					syntaxError((*mainEntery).Lines, expect.Pos, 0, "Unknown expect code \"%v\"", code[0])
				}
			}
		} else {
			offset := len("invoke " + invokeName + " ")
			knownEndPoints := make([]string, 0)
			for k := range namedEndpointes {
				knownEndPoints = append(knownEndPoints, k)
			}
			invalidParameter((*mainEntery).Lines, invoke.Pos, offset, "Endpoint \"%s\" not found for Invoke \"%s\"\nKnown endpoints:\n%s\n", invoke.EndPoint, invokeName, strings.Join(knownEndPoints, "\n"))
		}
	}
	if mainEntery.Warnings+mainEntery.Warnings > 0 {
		fmt.Printf("Test done with %d warning(s) and %d ignoration(s) \n", mainEntery.Warnings, mainEntery.Ignores)
	}
	println("✅ All tests passed as expected 😎")
}

func fatal(lines *[]string, pos lexer.Position, offset int, msg string, a ...interface{}) {
	line := (*lines)[pos.Line-1]
	newFormat := msg + "\nRelated line on file: %s:%d\n%s\n"
	if offset > 0 {
		newFormat += strings.Repeat(" ", offset) + "^\n"
	}

	a = append(a, pos.Filename, pos.Line, line)

	//println("MSG IS :", newFormat)

	//for _, v := range a {
	//	fmt.Printf("a: %v\n", v)
	//}
	fmt.Printf(newFormat, a...)
}

func syntaxError(lines *[]string, pos lexer.Position, offset int, msg string, a ...interface{}) {
	fmt.Println("Something went wrong!")

	fatal(lines, pos, offset, msg, a...)
	os.Exit(2)
}

func testFailed(testEntery *Entry, invoke *Invoke, expect *Expect, offset int, msg string, a ...interface{}) string {
	invokeCondition := NewInvokeCondition(*expect.OnFail, expect, fmt.Sprintf(msg, a...))
	colorFn := color.Red
	severityStr := "Panic"
	failSign := "⛔"
	switch invokeCondition.Condition {
	case InvokeDoneWithIgnores:
		{
			colorFn = color.Yellow
			failSign = "‼️"
			testEntery.Warnings += 1
		}
	case InvokeDoneWithWarnings:
		{
			colorFn = color.HiYellow
			failSign = "❕"
			testEntery.Ignores += 1
		}
		//case InvokeFailed:
		//Done already
	}

	colorFn("Test \"%s\" failed !  %s", testEntery.TestName, failSign)
	colorFn("Description: %s", testEntery.Description)
	if invoke.Goal != "" {
		colorFn("Goal: %s", invoke.Goal)
	}
	fatal(testEntery.Lines, expect.Pos, offset, severityStr+": "+msg, a...)

	expect.Invoke.Conditions = append(expect.Invoke.Conditions, invokeCondition)

	//Warn and Ignore will not break the test follow
	if severityStr == "Warn" || severityStr == "Ignore" {
		return severityStr
	}

	os.Exit(3)
	//We get out I knew, but needed to compile
	return ""
}

func invalidParameter(lines *[]string, pos lexer.Position, offset int, msg string, a ...interface{}) {
	fmt.Println("Invalid code error :")
	fatal(lines, pos, offset, msg, a...)
	os.Exit(2)
}

func typeOf(val interface{}) string {
	return fmt.Sprintf("%T", val)
}
