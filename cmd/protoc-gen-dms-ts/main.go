package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"unicode"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func relativeNamespaceImportPath(a, b string) string {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	sharedIndex := -1
	for idx, aPart := range aParts {
		if len(bParts) <= idx {
			break
		}

		bPart := bParts[idx]
		if aPart == bPart {
			sharedIndex = idx
		} else {
			break
		}
	}

	return fmt.Sprintf("../%s", strings.Join(bParts[sharedIndex+1:], "/")) + ".ts"
}

func camelToSnake(camel string) string {
	parts := []string{}
	current := ""
	for _, char := range camel {
		if current != "" && unicode.IsUpper(char) {
			parts = append(parts, current)
			current = string(unicode.ToLower(char))
		} else {
			current = current + string(unicode.ToLower(char))
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return strings.Join(parts, "_")
}

type File struct {
	namespace    string
	lines        []string
	indent       int
	imports      map[string]map[string]struct{}
	complexTypes map[string]struct{}
	needLib      bool
}

func (f *File) Write(data string, args ...interface{}) {
	if len(f.lines) == 0 {
		f.lines = append(f.lines, "")
	}
	f.lines[len(f.lines)-1] = f.lines[len(f.lines)-1] + fmt.Sprintf(data, args...)
}

func (f *File) WriteLine(line string, args ...interface{}) {
	f.lines = append(f.lines, strings.Repeat("  ", f.indent)+fmt.Sprintf(line, args...))
}

func (f *File) Content() string {
	return strings.Join(f.lines, "\n")
}

func (f *File) AddImport(namespace string, target string) {
	if _, ok := f.imports[namespace]; !ok {
		f.imports[namespace] = map[string]struct{}{}
	}

	f.imports[namespace][target] = struct{}{}
}

func protoTypeToTypeScript(t descriptorpb.FieldDescriptorProto_Type) string {
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return "number"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "boolean"
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		fallthrough
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "string"
	}
	panic(fmt.Sprintf("t is %v", t))
}

func generateComplexMessageType(file *File, message *descriptorpb.DescriptorProto, namespace string) {
	file.WriteLine("export namespace %s {", *message.Name)
	file.indent += 1

	for _, enum := range message.EnumType {
		generateEnumType(file, enum)
	}

	for _, nested := range message.NestedType {
		if len(nested.NestedType) == 0 && len(nested.EnumType) == 0 {
			generateSimpleMessageType(file, nested, "")
		} else {
			generateComplexMessageType(file, nested, namespace+*message.Name+".")
		}
	}

	generateSimpleMessageType(file, message, "Inner")

	file.indent -= 1
	file.WriteLine("};\n")
}

func generateSimpleMessageType(file *File, message *descriptorpb.DescriptorProto, nameOverride string) {
	if nameOverride == "" {
		nameOverride = *message.Name
	}
	file.WriteLine("export type %s = {", nameOverride)
	file.indent += 1

	oneOfs := make([]*descriptorpb.FieldDescriptorProto, 0)

	for _, field := range message.Field {
		if field.OneofIndex != nil {
			oneOfs = append(oneOfs, field)
			continue
		}

		name := field.GetJsonName()
		if field.GetProto3Optional() {
			name = name + "?"
		}

		if field.TypeName == nil {
			file.WriteLine("%s: %s;", name, protoTypeToTypeScript(*field.Type))
		} else {
			typeName := generateTypeReference(file, *field.TypeName)
			file.WriteLine("%s: %s;", name, typeName)
		}
	}
	file.indent -= 1

	if len(oneOfs) > 0 {
		file.WriteLine("} & (")

		for idx := range message.OneofDecl {
			for _, field := range oneOfs {
				if int(*field.OneofIndex) != idx {
					continue
				}

				name := field.GetJsonName()
				if field.GetProto3Optional() {
					name = name + "?"
				}

				if field.TypeName == nil {
					file.WriteLine(" | ({ %s: %s })", name, protoTypeToTypeScript(*field.Type))
				} else {
					typeName := generateTypeReference(file, *field.TypeName)
					file.WriteLine(" | ({ %s: %s })", name, typeName)
				}
			}
		}
		file.WriteLine(");\n")
	} else {
		file.WriteLine("};\n")
	}
}

func generateEnumType(file *File, enum *descriptorpb.EnumDescriptorProto) {
	enumPrefix := strings.ToUpper(camelToSnake(*enum.Name)) + "_"
	file.WriteLine("export enum %s {", *enum.Name)
	for _, value := range enum.Value {
		name := *value.Name
		if strings.HasPrefix(name, enumPrefix) {
			name = name[len(enumPrefix):]
		}
		file.WriteLine("  %s = %v,", name, *value.Number)
	}
	file.WriteLine("};\n")
}

func generateTypeReference(file *File, typeName string) string {
	if _, ok := file.complexTypes[typeName]; ok {
		typeName = typeName + ".Inner"
	}

	if strings.HasPrefix(typeName, file.namespace) {
		typeName = typeName[len(file.namespace):]
	} else {
		typeNameParts := strings.Split(typeName, ".")
		namespace := strings.Join(typeNameParts[:len(typeNameParts)-1], ".")
		file.AddImport(namespace, typeNameParts[len(typeNameParts)-1])
		typeName = typeNameParts[len(typeNameParts)-1]
	}
	return typeName
}

func generateServiceStub(file *File, service *descriptorpb.ServiceDescriptorProto) {
	file.WriteLine("export interface %sStub {", *service.Name)
	for _, method := range service.Method {
		inputType := generateTypeReference(file, *method.InputType)
		outputType := generateTypeReference(file, *method.OutputType)

		if method.GetServerStreaming() {
			file.WriteLine("  %s(request: %s, cb: (data: %s) => unknown): Promise<void>;", method.GetName(), inputType, outputType)
		} else {
			file.WriteLine("  %s(request: %s): Promise<%s>;", method.GetName(), inputType, outputType)
		}
	}
	file.WriteLine("};\n")
}

func generateServiceClient(file *File, service *descriptorpb.ServiceDescriptorProto) {
	file.needLib = true

	file.WriteLine("export class %s implements %sStub {", *service.Name, *service.Name)
	file.indent += 1
	file.WriteLine("constructor(public executor: GRPCExecutor) {}")
	for _, method := range service.Method {
		inputType := generateTypeReference(file, *method.InputType)
		outputType := generateTypeReference(file, *method.OutputType)

		if method.GetServerStreaming() {
			file.WriteLine("%s(request: %s, cb: (data: %s) => unknown): Promise<void> {", method.GetName(), inputType, outputType)
			file.WriteLine("  return this.executor.stream(\"%s%s.%s\", request, cb);", file.namespace[1:], *service.Name, method.GetName())
		} else {
			file.WriteLine("%s(request: %s): Promise<%s> {", method.GetName(), inputType, outputType)
			file.WriteLine("  return this.executor.invoke(\"%s%s.%s\", request);", file.namespace[1:], *service.Name, method.GetName())
		}
		file.WriteLine("}")

	}
	file.indent -= 1
	file.WriteLine("};\n")
}

func collectComplexTypes(complexTypes map[string]struct{}, message *descriptorpb.DescriptorProto, namespace string) {
	if len(message.EnumType) != 0 || len(message.NestedType) != 0 {
		complexTypes[namespace+message.GetName()] = struct{}{}
	}
	for _, nestedType := range message.NestedType {
		collectComplexTypes(complexTypes, nestedType, namespace+message.GetName()+".")
	}
}

func generateFile(complexTypes map[string]struct{}, file *descriptorpb.FileDescriptorProto) *pluginpb.CodeGeneratorResponse_File {
	name := strings.Join(strings.Split(file.GetPackage(), "."), "/") + ".ts"

	var result File
	result.namespace = "." + file.GetPackage() + "."
	result.imports = map[string]map[string]struct{}{}
	result.complexTypes = complexTypes

	for _, enum := range file.EnumType {
		generateEnumType(&result, enum)
	}

	for _, message := range file.MessageType {
		// This is a complex message and will be generated as a namespace
		if len(message.EnumType) == 0 && len(message.NestedType) == 0 {
			generateSimpleMessageType(&result, message, "")
		} else {
			generateComplexMessageType(&result, message, result.namespace)
		}
	}

	for _, service := range file.Service {
		generateServiceStub(&result, service)
		generateServiceClient(&result, service)
	}

	// generate imports
	header := []string{}
	for name, imports := range result.imports {
		importNames := []string{}
		for importName := range imports {
			importNames = append(importNames, importName)
		}
		importPath := relativeNamespaceImportPath(result.namespace, name)
		header = append(header, fmt.Sprintf("import { %s } from \"%s\";", strings.Join(importNames, ", "), importPath))
	}

	if result.needLib {
		header = append(header, "import { GRPCExecutor } from \"@sdk/grpc.ts\";")
	}

	content := strings.Join(header, "\n") + "\n\n" + result.Content()
	return &pluginpb.CodeGeneratorResponse_File{
		Name:    &name,
		Content: &content,
	}
}

func generateFiles(names []string, files map[string]*descriptorpb.FileDescriptorProto) []*pluginpb.CodeGeneratorResponse_File {
	result := make([]*pluginpb.CodeGeneratorResponse_File, 0)
	complexTypes := make(map[string]struct{})

	for _, fileName := range names {
		for _, message := range files[fileName].MessageType {
			collectComplexTypes(complexTypes, message, "."+files[fileName].GetPackage()+".")
		}
	}

	for _, fileName := range names {
		result = append(result, generateFile(complexTypes, files[fileName]))
	}

	return result
}

func main() {
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	request := &pluginpb.CodeGeneratorRequest{}
	err = proto.Unmarshal(data, request)
	if err != nil {
		panic(err)
	}

	files := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, file := range request.ProtoFile {
		files[*file.Name] = file
	}

	generatedFiles := generateFiles(request.FileToGenerate, files)

	supportedFeatures := uint64(1)
	response := &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: &supportedFeatures,
		File:              generatedFiles,
	}
	data, err = proto.Marshal(response)
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(data)
}
