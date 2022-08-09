package grpcrunner

import (
	"fmt"

	"github.com/fullstorydev/grpcurl"
	"github.com/golang/protobuf/proto" //lint:ignore SA1019 we have to import this because it appears in exported API
	"github.com/jhump/protoreflect/desc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type TRPCField struct {
	Desc  *desc.FieldDescriptor `json:"-"`
	Value interface{}           `json:"value"`
}

type TRPCMessage = map[string]TRPCField

type TRPCHandler struct {
	VerbosityLevel   int
	ResponseHeaders  metadata.MD
	Descriptor       grpcurl.DescriptorSource
	MethodDescriptor *desc.MethodDescriptor
	ResponseData     []proto.Message
	Trailers         metadata.MD
	Status           *status.Status
	NumRequests      int
	NumResponses     int
}

func (handler *TRPCHandler) OnResolveMethod(descriptor *desc.MethodDescriptor) {
	handler.MethodDescriptor = descriptor
}

func (handler *TRPCHandler) OnSendHeaders(rqHeaders metadata.MD) {
	if handler.VerbosityLevel > 0 {
		fmt.Println("Headers sent: \n", rqHeaders)
	}
	handler.NumRequests++
}

func (handler *TRPCHandler) OnReceiveHeaders(rsHeaders metadata.MD) {
	handler.ResponseHeaders = rsHeaders
}

func (handler *TRPCHandler) OnReceiveResponse(data proto.Message) {
	handler.ResponseData = append(handler.ResponseData, data)
	handler.NumResponses++
}

func (handler *TRPCHandler) OnReceiveTrailers(status *status.Status, metadata metadata.MD) {
	handler.Trailers = metadata
	handler.Status = status
}
