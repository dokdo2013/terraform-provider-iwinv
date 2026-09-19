package main

import (
	"context"
	"flag"
	"log"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Run with debugger reattach support")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/dokdo2013/iwinv", Debug: *debug, ProtocolVersion: 6,
	})
	if err != nil {
		log.Fatal(err)
	}
}
