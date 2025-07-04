package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/mm-aranda/terraform-provider-mirage/provider"
)

// The 'go build' command will automatically populate this value.
// For local development, it will be "dev".
var version = "dev"

func main() {
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		// This address must match the one in your Terraform configurations.
		Address: "localhost/my-org/mirage",
	})

	if err != nil {
		log.Fatal(err.Error())
	}
}
