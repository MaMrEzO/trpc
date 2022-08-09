// Command grpcurl makes gRPC requests (a la cURL, but HTTP/2). It can use a supplied descriptor
// file, protobuf sources, or service reflection to translate JSON or text request data into the
// appropriate protobuf messages and vice versa for presenting the response contents.
package grpcrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	//"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"

	//"google.golang.org/protobuf/types/descriptorpb"

	// Register gzip compressor so compressed responses will work
	_ "google.golang.org/grpc/encoding/gzip"
	// Register xds so xds and xds-experimental resolver schemes work
	_ "google.golang.org/grpc/xds"

	"github.com/fullstorydev/grpcurl"
)

// To avoid confusion between program error codes and the gRPC resonse
// status codes 'Cancelled' and 'Unknown', 1 and 2 respectively,
// the response status codes emitted use an offest of 64
const statusCodeOffset = 64

const no_version = "beta build"

var version = no_version

type RunParams struct {
	//server:port
	Target    string
	Ctx       context.Context
	Plaintext bool
	Insecure  bool
	CACert    string
	Cert      string
	Key       string
	Authority string

	ReflHeaders MultiString
	Protoset    MultiString
	ProtoFiles  MultiString
	ImportPaths MultiString

	AddlHeaders MultiString
	RPCHeaders  MultiString
	PrefixPath  string
	// ReflectionPrefixPath string
	ServiceName string
	MethodName  string
	UserAgent   string
	Data        map[string]interface{}

	AllowUnknownFields bool

	FormatError bool

	ConnectTimeout float64

	KeepaliveTime float64
	MaxTime       float64

	ExpandHeaders bool

	MaxMessagSize int
	//The maximum encoded size of a response message, in bytes, that grpcurl
	//will accept. If not specified, defaults to 4,194,304 (4 megabytes).`))
	EmitDefaults bool
	//Emit default values for JSON-encoded responses.`))
	MsgTemplate bool
	//When describing messages, show a template of input data.`))

	Verbose bool
	//Enable verbose output.`))
	VeryVerbose bool
	//Enable very verbose output.`))

	ServerName string
	// Override server name when validating TLS certificate. This flag is
	// ignored if -plaintext or -insecure is used.
	// NOTE: Prefer -authority. This flag may be removed in the future. It is
	// an error to use both -authority and -servername (though this will be
	// permitted if they are both set to the same value, to increase backwards
	// compatibility with earlier releases that allowed both to be set).`))
}

var (
	exit = os.Exit

	isUnixSocket func() bool // nil when run on non-unix platform

	//formatError = flags.Bool("format-error", false, prettify(`
	//	When a non-zero status is returned, format the response using the
	//	value set by the -format flag .`))
	//protosetOut = flags.String("protoset-out", "", prettify(`
	//	The name of a file to be written that will contain a FileDescriptorSet
	//	proto. With the list and describe verbs, the listed or described
	//	elements and their transitive dependencies will be written to the named
	//	file if this option is given. When invoking an RPC and this option is
	//	given, the method being invoked and its transitive dependencies will be
	//	included in the output file.`))
	reflection = optionalBoolFlag{val: true}
)

//func init() {
//	flags.Var(&addlHeaders, "H", prettify(`
//		Additional headers in 'name: value' format. May specify more than one
//		via multiple flags. These headers will also be included in reflection
//		requests to a server.`))
//	flags.Var(&rpcHeaders, "rpc-header", prettify(`
//		Additional RPC headers in 'name: value' format. May specify more than
//		one via multiple flags. These headers will *only* be used when invoking
//		the requested RPC method. They are excluded from reflection requests.`))
//	flags.Var(&reflHeaders, "reflect-header", prettify(`
//		Additional reflection headers in 'name: value' format. May specify more
//		than one via multiple flags. These headers will *only* be used during
//		reflection requests and will be excluded when invoking the requested RPC
//		method.`))
//	flags.Var(&protoset, "protoset", prettify(`
//		The name of a file containing an encoded FileDescriptorSet. This file's
//		contents will be used to determine the RPC schema instead of querying
//		for it from the remote server via the gRPC reflection API. When set: the
//		'list' action lists the services found in the given descriptors (vs.
//		those exposed by the remote server), and the 'describe' action describes
//		symbols found in the given descriptors. May specify more than one via
//		multiple -protoset flags. It is an error to use both -protoset and
//		-proto flags.`))
//	flags.Var(&protoFiles, "proto", prettify(`
//		The name of a proto source file. Source files given will be used to
//		determine the RPC schema instead of querying for it from the remote
//		server via the gRPC reflection API. When set: the 'list' action lists
//		the services found in the given files and their imports (vs. those
//		exposed by the remote server), and the 'describe' action describes
//		symbols found in the given files. May specify more than one via multiple
//		-proto flags. Imports will be resolved using the given -import-path
//		flags. Multiple proto files can be specified by specifying multiple
//		-proto flags. It is an error to use both -protoset and -proto flags.`))
//	flags.Var(&importPaths, "import-path", prettify(`
//		The path to a directory from which proto sources can be imported, for
//		use with -proto flags. Multiple import paths can be configured by
//		specifying multiple -import-path flags. Paths will be searched in the
//		order given. If no import paths are given, all files (including all
//		imports) must be provided as -proto flags, and grpcurl will attempt to
//		resolve all import statements from the set of file names given.`))
//	flags.Var(&reflection, "use-reflection", prettify(`
//		When true, server reflection will be used to determine the RPC schema.
//		Defaults to true unless a -proto or -protoset option is provided. If
//		-use-reflection is used in combination with a -proto or -protoset flag,
//		the provided descriptor sources will be used in addition to server
//		reflection to resolve messages and extensions.`))
//}

type MultiString []string

func (s *MultiString) String() string {
	return strings.Join(*s, ",")
}

func (s *MultiString) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Uses a file source as a fallback for resolving symbols and extensions, but
// only uses the reflection source for listing services
type compositeSource struct {
	reflection grpcurl.DescriptorSource
	file       grpcurl.DescriptorSource
}

func (cs compositeSource) ListServices() ([]string, error) {
	return cs.reflection.ListServices()
}

func (cs compositeSource) FindSymbol(fullyQualifiedName string) (desc.Descriptor, error) {
	d, err := cs.reflection.FindSymbol(fullyQualifiedName)
	if err == nil {
		return d, nil
	}
	return cs.file.FindSymbol(fullyQualifiedName)
}

func (cs compositeSource) AllExtensionsForType(typeName string) ([]*desc.FieldDescriptor, error) {
	exts, err := cs.reflection.AllExtensionsForType(typeName)
	if err != nil {
		// On error fall back to file source
		return cs.file.AllExtensionsForType(typeName)
	}
	// Track the tag numbers from the reflection source
	tags := make(map[int32]bool)
	for _, ext := range exts {
		tags[ext.GetNumber()] = true
	}
	fileExts, err := cs.file.AllExtensionsForType(typeName)
	if err != nil {
		return exts, nil
	}
	for _, ext := range fileExts {
		// Prioritize extensions found via reflection
		if !tags[ext.GetNumber()] {
			exts = append(exts, ext)
		}
	}
	return exts, nil
}

func Run(params RunParams) (*TRPCHandler, error) {

	// Do extra validation on arguments and figure out what user asked us to do.
	if params.ConnectTimeout < 0 {
		fail(nil, "The -connect-timeout argument must not be negative.")
	}
	if params.KeepaliveTime < 0 {
		fail(nil, "The -keepalive-time argument must not be negative.")
	}
	if params.MaxTime < 0 {
		fail(nil, "The -max-time argument must not be negative.")
	}
	if params.MaxMessagSize < 0 {
		fail(nil, "The -max-msg-sz argument must not be negative.")
	}
	if params.Plaintext && params.Insecure {
		fail(nil, "The -plaintext and -insecure arguments are mutually exclusive.")
	}
	if params.Plaintext && params.Cert != "" {
		fail(nil, "The -plaintext and -cert arguments are mutually exclusive.")
	}
	if params.Plaintext && params.Key != "" {
		fail(nil, "The -plaintext and -key arguments are mutually exclusive.")
	}
	if (params.Key == "") != (params.Cert == "") {
		fail(nil, "The -cert and -key arguments must be used together and both be present.")
	}

	//var list, describe, invoke bool
	//list = false
	//describe = false
	//invoke = true
	//if args[0] == "list" {
	//	list = true
	//	args = args[1:]
	//} else if args[0] == "describe" {
	//	describe = true
	//	args = args[1:]
	//} else {
	//	invoke = true
	//}

	verbosityLevel := 0
	if params.Verbose {
		verbosityLevel = 1
	}
	if params.VeryVerbose {
		verbosityLevel = 2
	}

	//var symbol string
	//if invoke {
	//	if len(args) == 0 {
	//		fail(nil, "Too few arguments.")
	//	}
	//	symbol = args[0]
	//	args = args[1:]
	//} else {
	//	if *data != "" {
	//		warn("The -d argument is not used with 'list' or 'describe' verb.")
	//	}
	//	if len(rpcHeaders) > 0 {
	//		warn("The -rpc-header argument is not used with 'list' or 'describe' verb.")
	//	}
	//	if len(args) > 0 {
	//		symbol = args[0]
	//		args = args[1:]
	//	}
	//}

	//if len(args) > 0 {
	//	fail(nil, "Too many arguments.")
	//}
	//if invoke && target == "" {
	//	fail(nil, "No host:port specified.")
	//}
	//if len(protoset) == 0 && len(protoFiles) == 0 && target == "" {
	//	fail(nil, "No host:port specified, no protoset specified, and no proto sources specified.")
	//}
	//if len(protoset) > 0 && len(reflHeaders) > 0 {
	//	warn("The -reflect-header argument is not used when -protoset files are used.")
	//}
	//if len(protoset) > 0 && len(protoFiles) > 0 {
	//	fail(nil, "Use either -protoset files or -proto files, but not both.")
	//}
	//if len(importPaths) > 0 && len(protoFiles) == 0 {
	//	warn("The -import-path argument is not used unless -proto files are used.")
	//}
	//if !reflection.val && len(protoset) == 0 && len(protoFiles) == 0 {
	//	fail(nil, "No protoset files or proto files specified and -use-reflection set to false.")
	//}

	//// Protoset or protofiles provided and -use-reflection unset
	if !reflection.set && (len(params.Protoset) > 0 || len(params.ProtoFiles) > 0) {
		reflection.val = false
	}

	ctx := context.Background()
	if params.MaxTime > 0 {
		timeout := time.Duration(params.MaxTime * float64(time.Second))
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	dial := func() *grpc.ClientConn {
		dialTime := 10 * time.Second
		if params.ConnectTimeout > 0 {
			dialTime = time.Duration(params.ConnectTimeout * float64(time.Second))
		}
		ctx, cancel := context.WithTimeout(ctx, dialTime)
		defer cancel()
		var opts []grpc.DialOption
		if params.KeepaliveTime > 0 {
			timeout := time.Duration(params.KeepaliveTime * float64(time.Second))
			opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:    timeout,
				Timeout: timeout,
			}))
		}
		if params.MaxMessagSize > 0 {
			opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(params.MaxMessagSize)))
		}
		var creds credentials.TransportCredentials
		if !params.Plaintext {
			tlsConf, err := grpcurl.ClientTLSConfig(params.Insecure, params.CACert, params.Cert, params.Key)
			if err != nil {
				fail(err, "Failed to create TLS config")
			}

			sslKeylogFile := os.Getenv("SSLKEYLOGFILE")
			if sslKeylogFile != "" {
				w, err := os.OpenFile(sslKeylogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
				if err != nil {
					fail(err, "Could not open SSLKEYLOGFILE %s", sslKeylogFile)
				}
				tlsConf.KeyLogWriter = w
			}

			creds = credentials.NewTLS(tlsConf)

			// can use either -servername or -authority; but not both
			if params.ServerName != "" && params.Authority != "" {
				if params.ServerName == params.Authority {
					warn("Both -servername and -authority are present; prefer only -authority.")
				} else {
					fail(nil, "Cannot specify different values for -servername and -authority.")
				}
			}
			overrideName := params.ServerName
			if overrideName == "" {
				overrideName = params.Authority
			}

			if overrideName != "" {
				opts = append(opts, grpc.WithAuthority(overrideName))
			}
		} else if params.Authority != "" {
			opts = append(opts, grpc.WithAuthority(params.Authority))
		}

		grpcurlUA := "trpc/" + version
		if version == no_version {
			grpcurlUA = "trpc/beta build"
		}
		if params.UserAgent != "" {
			grpcurlUA = params.UserAgent + " " + grpcurlUA
		}
		opts = append(opts, grpc.WithUserAgent(grpcurlUA))

		network := "tcp"
		if isUnixSocket != nil && isUnixSocket() {
			network = "unix"
		}

		//println("Dialing", network, params.Target)
		cc, err := grpcurl.BlockingDial(ctx, network, params.Target, creds, opts...)
		//cc, err := DirectDialContext(ctx, params.Target, params.PrefixPath, opts...)
		if err != nil {
			fail(err, "Failed to dial target host %q", params.Target)
		}
		return cc
	}
	//printFormattedStatus := func(w io.Writer, stat *status.Status, formatter grpcurl.Formatter) {
	//	formattedStatus, err := formatter(stat.Proto())
	//	if err != nil {
	//		fmt.Fprintf(w, "ERROR: %v", err.Error())
	//	}
	//	fmt.Fprint(w, formattedStatus)
	//}

	if params.ExpandHeaders {
		var err error
		params.AddlHeaders, err = grpcurl.ExpandHeaders(params.AddlHeaders)
		if err != nil {
			fail(err, "Failed to expand additional headers")
		}
		params.RPCHeaders, err = grpcurl.ExpandHeaders(params.RPCHeaders)
		if err != nil {
			fail(err, "Failed to expand rpc headers")
		}
		params.ReflHeaders, err = grpcurl.ExpandHeaders(params.ReflHeaders)
		if err != nil {
			fail(err, "Failed to expand reflection headers")
		}
	}

	var cc *grpc.ClientConn
	var descSource grpcurl.DescriptorSource
	var refClient *grpcreflect.Client
	var fileSource grpcurl.DescriptorSource
	if len(params.Protoset) > 0 {
		var err error
		fileSource, err = grpcurl.DescriptorSourceFromProtoSets(params.Protoset...)
		if err != nil {
			fail(err, "Failed to process proto descriptor sets.")
		}
	} else if len(params.ProtoFiles) > 0 {
		var err error
		fileSource, err = grpcurl.DescriptorSourceFromProtoFiles(params.ImportPaths, params.ProtoFiles...)
		if err != nil {
			fail(err, "Failed to process proto source files.")
		}
	}
	if reflection.val {
		println("Dialing for reflection")
		md := grpcurl.MetadataFromHeaders(append(params.AddlHeaders, params.ReflHeaders...))
		refCtx := metadata.NewOutgoingContext(ctx, md)
		cc = dial()
		println("Continue to refClient")
		refClient = grpcreflect.NewClient(refCtx, reflectpb.NewServerReflectionClient(RefClientConnFromConn(cc, params.PrefixPath)))
		println("Continue to descSource")
		reflSource := grpcurl.DescriptorSourceFromServer(ctx, refClient)

		if fileSource != nil {
			println("Continue to merge")

			descSource = compositeSource{reflSource, fileSource}
		} else {
			println("Continue to refSource")
			descSource = reflSource
		}
	} else {
		descSource = fileSource
	}

	// arrange for the RPCs to be cleanly shutdown
	reset := func() {
		if refClient != nil {
			refClient.Reset()
			refClient = nil
		}
		if cc != nil {
			cc.Close()
			//cc.(*DirectClientConn).Close()
			cc = nil
		}
	}
	defer reset()
	exit = func(code int) {
		// since defers aren't run by os.Exit...
		reset()
		os.Exit(code)
	}

	// Invoke an RPC
	if cc == nil {
		cc = dial()
	}

	data, err := json.Marshal(params.Data)

	var in io.Reader
	in = strings.NewReader(string(data))

	// if not verbose output, then also include record delimiters
	// between each message, so output could potentially be piped
	// to another grpcurl process
	includeSeparators := verbosityLevel == 0
	options := grpcurl.FormatOptions{
		EmitJSONDefaultFields: params.EmitDefaults,
		IncludeTextSeparator:  includeSeparators,
		AllowUnknownFields:    params.AllowUnknownFields,
	}
	//rf, formatter, err := grpcurl.RequestParserAndFormatter(grpcurl.Format("json"), descSource, in, options)
	rf, _, err := grpcurl.RequestParserAndFormatter(grpcurl.Format("json"), descSource, in, options)
	if err != nil {
		fail(err, "Failed to construct request parser and formatter for json")
	}
	//h := &grpcurl.DefaultEventHandler{
	//	Out:            os.Stdout,
	//	Formatter:      formatter,
	//	VerbosityLevel: verbosityLevel,
	//}

	h := &TRPCHandler{
		Descriptor: descSource,
	}

	symbol := fmt.Sprintf("%s/%s", params.ServiceName, params.MethodName)

	err = grpcurl.InvokeRPC(ctx, descSource, RefClientConnFromConn(cc, params.PrefixPath) /*params.PrefixPath,*/, symbol, append(params.AddlHeaders, params.RPCHeaders...), h, rf.Next)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok && params.FormatError {
			h.Status = errStatus
		} else {
			fail(err, "Error invoking method %q", symbol)
			return nil, err
		}
	}
	reqSuffix := ""
	respSuffix := ""
	reqCount := rf.NumRequests()
	if reqCount != 1 {
		reqSuffix = "s"
	}
	if h.NumResponses != 1 {
		respSuffix = "s"
	}
	if verbosityLevel > 0 {
		fmt.Printf("Sent %d request%s and received %d response%s\n", reqCount, reqSuffix, h.NumResponses, respSuffix)
	}
	if h.Status.Code() != codes.OK {
		//if params.FormatError {
		//	printFormattedStatus(os.Stderr, h.Status, formatter)
		//} else {
		//	grpcurl.PrintStatus(os.Stderr, h.Status, formatter)
		//}
		//exit(statusCodeOffset + int(h.Status.Code()))
	}
	return h, nil
}

func prettify(docString string) string {
	parts := strings.Split(docString, "\n")

	// cull empty lines and also remove trailing and leading spaces
	// from each line in the doc string
	j := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts[j] = part
		j++
	}

	return strings.Join(parts[:j], "\n"+indent())
}

func warn(msg string, args ...interface{}) {
	msg = fmt.Sprintf("Warning: %s\n", msg)
	fmt.Fprintf(os.Stderr, msg, args...)
}

func fail(err error, msg string, args ...interface{}) {
	if err != nil {
		msg += ": %v"
		args = append(args, err)
	}
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		exit(1)
	} else {
		// nil error means it was CLI usage issue
		fmt.Fprintf(os.Stderr, "Try '%s -help' for more details.\n", os.Args[0])
		exit(2)
	}
}

//func writeProtoset(descSource grpcurl.DescriptorSource, symbols ...string) error {
//	if *protosetOut == "" {
//		return nil
//	}
//	f, err := os.Create(*protosetOut)
//	if err != nil {
//		return err
//	}
//	defer f.Close()
//	return grpcurl.WriteProtoset(f, descSource, symbols...)
//}

type optionalBoolFlag struct {
	set, val bool
}

func (f *optionalBoolFlag) String() string {
	if !f.set {
		return "unset"
	}
	return strconv.FormatBool(f.val)
}

func (f *optionalBoolFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	f.set = true
	f.val = v
	return nil
}

func (f *optionalBoolFlag) IsBoolFlag() bool {
	return true
}
