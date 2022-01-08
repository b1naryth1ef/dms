package main

import (
	"flag"

	"github.com/b1naryth1ef/dms"
)

var endpoint = flag.String("endpoint", "localhost:50051", "target grpc server endpoint")
var bind = flag.String("bind", "localhost:6975", "server bind target")

func main() {
	flag.Parse()
	server := &dms.HTTPServer{
		Endpoint: *endpoint,
	}
	server.Run(*bind)
}
