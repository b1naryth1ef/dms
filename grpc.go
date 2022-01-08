package dms

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type GRPCProxy struct {
	methods      map[string]*descriptorpb.MethodDescriptorProto
	messageTypes map[string]protoreflect.MessageType
}

func NewGRPCProxy(descriptorData []byte) (*GRPCProxy, error) {
	var data descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(descriptorData, &data); err != nil {
		return nil, err
	}

	messageTypes := make(map[string]protoreflect.MessageType)
	methods := make(map[string]*descriptorpb.MethodDescriptorProto)
	for _, file := range data.File {
		reflectFile, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
		if err != nil {
			return nil, err
		}
		protoregistry.GlobalFiles.RegisterFile(reflectFile)

		for _, service := range file.Service {
			for _, method := range service.Method {
				methods[fmt.Sprintf("%s.%s.%s", *file.Package, *service.Name, *method.Name)] = method
			}
		}

		for i := 0; i < reflectFile.Messages().Len(); i++ {
			message := reflectFile.Messages().Get(i)
			messageTypes[string(message.FullName())] = dynamicpb.NewMessageType(message)
		}
	}
	return &GRPCProxy{
		methods:      methods,
		messageTypes: messageTypes,
	}, nil
}
