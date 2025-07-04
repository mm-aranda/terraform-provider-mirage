package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = &MirageProvider{}

type MirageProvider struct {
	version string
}

func (p *MirageProvider) Metadata(_ context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "mirage"
	resp.Version = p.version
}

func (p *MirageProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Mirage Provider for managing resources in the Mirage ecosystem.",
	}
}

func (p *MirageProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
}

func (p *MirageProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDagGeneratorResource,
	}
}

func (p *MirageProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MirageProvider{
			version: version,
		}
	}
}
