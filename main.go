package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Run with debugger reattach support")
	showVersion := flag.Bool("version", false, "Print provider version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/dokdo2013/iwinv", Debug: *debug, ProtocolVersion: 6,
	})
	if err != nil {
		log.Fatal(err)
	}
}
