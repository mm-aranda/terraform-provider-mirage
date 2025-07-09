package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/mm-aranda/terraform-provider-mirage/internal/client"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &dagGeneratorResource{}
	_ resource.ResourceWithImportState = &dagGeneratorResource{}
)

func NewDagGeneratorResource() resource.Resource {
	return &dagGeneratorResource{}
}

type dagGeneratorResource struct {
	dagGenService *client.DagGeneratorService
}

type dagGeneratorResourceModel struct {
	DagGeneratorBackendURL   types.String `tfsdk:"dag_generator_backend_url"`
	TemplateGCSPath          types.String `tfsdk:"template_gcs_path"`
	TemplateContent          types.String `tfsdk:"template_content"`
	TargetGCSPath            types.String `tfsdk:"target_gcs_path"`
	ContextJSON              types.String `tfsdk:"context_json"`
	GeneratedFileChecksum    types.String `tfsdk:"generated_file_checksum"`
	GCSGenerationNumber      types.String `tfsdk:"gcs_generation_number"`
	ID                       types.String `tfsdk:"id"`
	UseGCPServiceAccountAuth types.Bool   `tfsdk:"use_gcp_service_account_auth"`
}

func (r *dagGeneratorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
}

func (r *dagGeneratorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dag_generator"
}

func (r *dagGeneratorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a generated file (e.g., an Airflow DAG) in Google Cloud Storage.",
		Attributes: map[string]schema.Attribute{
			"dag_generator_backend_url": schema.StringAttribute{
				Description: "The base URL of the backend service for this specific resource.",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "The GCS path of the generated file, used as the resource ID.",
				Computed:    true,
			},
			"template_gcs_path": schema.StringAttribute{
				Description: "The full gs:// path to the source Jinja2 template.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"template_content": schema.StringAttribute{
				Description: "The content of the local template file.",
				Optional:    true,
			},
			"target_gcs_path": schema.StringAttribute{
				Description: "The full gs:// path for the generated output file.",
				Required:    true,
			},
			"context_json": schema.StringAttribute{
				Description: "A JSON string representing the dynamic context for the template.",
				Required:    false,
				Optional:    true,
			},
			"generated_file_checksum": schema.StringAttribute{
				Description: "The CRC32C checksum of the generated file in GCS.",
				Computed:    true,
			},
			"gcs_generation_number": schema.StringAttribute{
				Description: "The GCS generation number of the generated file.",
				Computed:    true,
			},
			"use_gcp_service_account_auth": schema.BoolAttribute{
				Description: "If true, authenticate requests to the backend using the machine's GCP service account.",
				Optional:    true,
				Computed:    false,
			},
		},
	}
}

func (r *dagGeneratorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dagGeneratorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gcsPath := plan.TemplateGCSPath.ValueString()
	content := plan.TemplateContent.ValueString()

	if (gcsPath == "" && content == "") || (gcsPath != "" && content != "") {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Exactly one of `template_gcs_path` or `template_content` must be specified.",
		)
		return
	}

	// Initialize API client and service using backend_url from the plan
	apiClient := client.NewDagGeneratorAPIClientWithAuth(plan.DagGeneratorBackendURL.ValueString(), plan.UseGCPServiceAccountAuth.ValueBool())
	dagGenService := &client.DagGeneratorService{Client: apiClient}

	contextJSON := plan.ContextJSON.ValueString()
	generationResult, err := dagGenService.Generate(ctx, gcsPath, content, plan.TargetGCSPath.ValueString(), contextJSON)
	if err != nil {
		resp.Diagnostics.AddError("Failed to generate DAG", err.Error())
		return
	}

	plan.ID = plan.TargetGCSPath
	plan.GeneratedFileChecksum = basetypes.NewStringValue(generationResult.Checksum)
	plan.GCSGenerationNumber = basetypes.NewStringValue(generationResult.Generation)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dagGeneratorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dagGeneratorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Initialize API client and service using backend_url from state
	apiClient := client.NewDagGeneratorAPIClientWithAuth(state.DagGeneratorBackendURL.ValueString(), state.UseGCPServiceAccountAuth.ValueBool())
	dagGenService := &client.DagGeneratorService{Client: apiClient}

	status, err := dagGenService.GetStatus(ctx, state.TargetGCSPath.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.Diagnostics.AddWarning("File not found", "The resource no longer exists in the backend and will be removed from the state.")
			resp.State.RemoveResource(ctx)
		} else {
			// For any other error (e.g., network), report it and stop.
			resp.Diagnostics.AddError("Failed to read resource status", err.Error())
		}
		return
	}

	state.GeneratedFileChecksum = basetypes.NewStringValue(status.Checksum)
	state.GCSGenerationNumber = basetypes.NewStringValue(status.Generation)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dagGeneratorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dagGeneratorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gcsPath := plan.TemplateGCSPath.ValueString()
	content := plan.TemplateContent.ValueString()

	if (gcsPath == "" && content == "") || (gcsPath != "" && content != "") {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Exactly one of `template_gcs_path` or `template_content` must be specified.",
		)
		return
	}

	// Initialize API client and service using backend_url from the plan
	apiClient := client.NewDagGeneratorAPIClientWithAuth(plan.DagGeneratorBackendURL.ValueString(), plan.UseGCPServiceAccountAuth.ValueBool())
	dagGenService := &client.DagGeneratorService{Client: apiClient}

	contextJSON := plan.ContextJSON.ValueString()
	generationResult, err := dagGenService.Generate(ctx, gcsPath, content, plan.TargetGCSPath.ValueString(), contextJSON)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update DAG", err.Error())
		return
	}

	plan.ID = plan.TargetGCSPath
	plan.GeneratedFileChecksum = basetypes.NewStringValue(generationResult.Checksum)
	plan.GCSGenerationNumber = basetypes.NewStringValue(generationResult.Generation)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dagGeneratorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dagGeneratorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Initialize API client and service using backend_url from state
	apiClient := client.NewDagGeneratorAPIClientWithAuth(state.DagGeneratorBackendURL.ValueString(), state.UseGCPServiceAccountAuth.ValueBool())
	dagGenService := &client.DagGeneratorService{Client: apiClient}

	err := dagGenService.Delete(ctx, state.TargetGCSPath.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete DAG", err.Error())
		return
	}
}

func (r *dagGeneratorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
