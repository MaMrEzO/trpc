package trpc_marshal

import (
	"trpc/grpcrunner"

	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

func marshalField(value any, field *desc.FieldDescriptor) any {
	if field.GetType() == dpb.FieldDescriptorProto_TYPE_ENUM {
		field.GetEnumType().FindValueByNumber(value.(int32)).GetName()
	}
	return value
}

func marshalRepeated(msg []interface{}, field *desc.FieldDescriptor) []any {
	repeated := make([]interface{}, 0)
	if field.GetType() == dpb.FieldDescriptorProto_TYPE_MESSAGE {
		fields := field.GetMessageType().GetFields()
		for _, v := range msg {
			repeated = append(repeated, marshalMsg(v.(*dynamic.Message), fields))
		}
	} else if field.GetType() == dpb.FieldDescriptorProto_TYPE_ENUM {
		for _, v := range msg {
			repeated = append(repeated, marshalField(v, field))
		}
	}
	return repeated
}

func marshalMsg(msg *dynamic.Message, fields []*desc.FieldDescriptor) map[string]any {
	marshalled := make(map[string]any, 0)
	for _, field := range fields {
		if field.IsRepeated() {
			message := msg.GetField(field).([]interface{})
			marshalled[field.GetName()] = marshalRepeated(message, field)
		} else {
			if msg.HasField(field) {
				marshalled[field.GetName()] = marshalField(msg.GetField(field), field)
			} else {
				marshalled[field.GetName()] = nil
			}
		}
	}
	return marshalled
}

func RPCMessageToMap(handler grpcrunner.TRPCHandler) map[string]any {
	message := handler.ResponseData[0]

	mdRes := handler.MethodDescriptor.GetOutputType()
	reflMsg, _ := dynamic.AsDynamicMessage(message)

	return marshalMsg(reflMsg, mdRes.GetFields())
}
