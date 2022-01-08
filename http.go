package dms

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/alioygur/gores"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

//go:generate bin/gen-proto rust-server/protos/
//go:embed descriptors.bin
var descriptors []byte

var protoJSONMarshalOpts = protojson.MarshalOptions{
	UseEnumNumbers:  true,
	EmitUnpopulated: true,
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type HTTPServer struct {
	proxy    *GRPCProxy
	Endpoint string
}

func (h *HTTPServer) GetHealth(w http.ResponseWriter, r *http.Request) error {
	gores.String(w, http.StatusOK, "OK")
	return nil
}

func (h *HTTPServer) GetStream(w http.ResponseWriter, r *http.Request) error {
	methodName := chi.URLParam(r, "*")

	method, ok := h.proxy.methods[methodName]
	if !ok {
		return errors.New("unknown method")
	}

	inputTypeName := *method.InputType
	if strings.HasPrefix(inputTypeName, ".") {
		inputTypeName = inputTypeName[1:]
	}

	inputTypeInst, ok := h.proxy.messageTypes[inputTypeName]
	if !ok {
		return errors.New("no inst")
	}

	outputTypeName := *method.OutputType
	if strings.HasPrefix(outputTypeName, ".") {
		outputTypeName = outputTypeName[1:]
	}

	outputTypeInst, ok := h.proxy.messageTypes[outputTypeName]
	if !ok {
		return errors.New("no output inst")
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	var initialMessage protoreflect.ProtoMessage
	if method.ClientStreaming == nil || *method.ClientStreaming == false {
		// read initial request data
		_, initialData, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		initialMessage = inputTypeInst.New().Interface()
		err = protojson.Unmarshal(initialData, initialMessage)
		if err != nil {
			return err
		}
	}

	client, err := grpc.Dial(h.Endpoint, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer client.Close()

	parts := strings.Split(methodName, ".")
	targetPath := fmt.Sprintf("/%s/%s", strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1])
	stream, err := client.NewStream(r.Context(), &grpc.StreamDesc{
		ServerStreams: true,
	}, targetPath)
	if err != nil {
		return err
	}

	var readMessage protoreflect.ProtoMessage
	readMessage = outputTypeInst.New().Interface()

	if initialMessage != nil {
		stream.SendMsg(initialMessage)
	}

	for {
		err := stream.RecvMsg(readMessage)
		if err != nil {
			return err
		}

		jsonData, err := protoJSONMarshalOpts.Marshal(readMessage)
		if err != nil {
			return err
		}

		err = conn.WriteMessage(websocket.TextMessage, jsonData)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (h *HTTPServer) Bind(fn func(w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err != nil {
			gores.Error(w, http.StatusInternalServerError, fmt.Sprintf("error: %s", err))
		}
	}
}

func (h *HTTPServer) call(method string, input, output interface{}) error {
	conn, err := grpc.Dial(h.Endpoint, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.Invoke(context.Background(), method, input, output)
	if err != nil {
		return err
	}

	return nil
}

func (h *HTTPServer) PostCall(w http.ResponseWriter, r *http.Request) error {
	methodName := chi.URLParam(r, "*")

	method, ok := h.proxy.methods[methodName]
	if !ok {
		return errors.New("bad service")
	}

	inputTypeName := *method.InputType
	if strings.HasPrefix(inputTypeName, ".") {
		inputTypeName = inputTypeName[1:]
	}

	inputTypeInst, ok := h.proxy.messageTypes[inputTypeName]
	if !ok {
		return errors.New("no inst")
	}

	outputTypeName := *method.OutputType
	if strings.HasPrefix(outputTypeName, ".") {
		outputTypeName = outputTypeName[1:]
	}

	outputTypeInst, ok := h.proxy.messageTypes[outputTypeName]
	if !ok {
		return errors.New("no output inst")
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body.Close()

	msg := inputTypeInst.New().Interface()
	err = protojson.Unmarshal(data, msg)
	if err != nil {
		return err
	}

	parts := strings.Split(methodName, ".")
	targetPath := fmt.Sprintf("/%s/%s", strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1])
	res := outputTypeInst.New().Interface()
	err = h.call(targetPath, msg, res)
	if err != nil {
		return err
	}

	resultJSON, err := protoJSONMarshalOpts.Marshal(res)
	if err != nil {
		return err
	}

	w.WriteHeader(200)
	w.Write(resultJSON)
	return nil
}

func (h *HTTPServer) Run(bind string) error {
	proxy, err := NewGRPCProxy(descriptors)
	if err != nil {
		return err
	}
	h.proxy = proxy

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Get("/health", h.Bind(h.GetHealth))
	r.Post("/call/*", h.Bind(h.PostCall))
	r.Get("/stream/*", h.Bind(h.GetStream))

	return http.ListenAndServe(bind, r)
}
