package provider

import (
	"context"
	"fmt"
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
	TemplateChecksum         types.String `tfsdk:"template_checksum"`
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
			"template_checksum": schema.StringAttribute{
				Description: "The CRC32C checksum of the template file in GCS.",
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

	// Store template checksum if using GCS template
	if gcsPath != "" {
		templateStatus, err := dagGenService.GetTemplateStatus(ctx, gcsPath)
		if err != nil {
			// Warning but don't fail
			resp.Diagnostics.AddWarning(
				"Could not get template status",
				fmt.Sprintf("Unable to get template status for %s: %v", gcsPath, err),
			)
			plan.TemplateChecksum = basetypes.NewStringValue("")
		} else {
			plan.TemplateChecksum = basetypes.NewStringValue(templateStatus.Checksum)
		}
	} else {
		plan.TemplateChecksum = basetypes.NewStringValue("")
	}

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

	// Update template checksum if using GCS template
	if state.TemplateGCSPath.ValueString() != "" {
		templateStatus, err := dagGenService.GetTemplateStatus(ctx, state.TemplateGCSPath.ValueString())
		if err != nil {
			// Warning but don't fail
			resp.Diagnostics.AddWarning(
				"Could not get template status",
				fmt.Sprintf("Unable to get template status for %s: %v", state.TemplateGCSPath.ValueString(), err),
			)
		} else {
			state.TemplateChecksum = basetypes.NewStringValue(templateStatus.Checksum)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dagGeneratorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dagGeneratorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state dagGeneratorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
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

	// Check if target_gcs_path has changed - if so, delete the old file first
	oldTargetPath := state.TargetGCSPath.ValueString()
	newTargetPath := plan.TargetGCSPath.ValueString()
	
	if oldTargetPath != newTargetPath && oldTargetPath != "" {
		// Delete the old file
		err := dagGenService.Delete(ctx, oldTargetPath)
		if err != nil {
			// Log warning but don't fail - the old file might already be gone
			resp.Diagnostics.AddWarning(
				"Failed to delete old file",
				fmt.Sprintf("Could not delete old file at %s: %v", oldTargetPath, err),
			)
		}
	}

	// Check if we need to regenerate due to template changes (only for GCS templates)
	shouldRegenerate := true
	if gcsPath != "" {
		// Check if template has been modified
		templateStatus, err := dagGenService.GetTemplateStatus(ctx, gcsPath)
		if err != nil {
			// If we can't check template status, proceed with regeneration
			resp.Diagnostics.AddWarning(
				"Could not check template status",
				fmt.Sprintf("Unable to check if template %s has been modified: %v. Proceeding with regeneration.", gcsPath, err),
			)
		} else {
			// Compare template checksum with what we have in state
			currentTemplateChecksum := templateStatus.Checksum
			storedTemplateChecksum := state.TemplateChecksum.ValueString()
			
			if currentTemplateChecksum != "" && storedTemplateChecksum != "" && currentTemplateChecksum == storedTemplateChecksum {
				// Template hasn't changed, check if other parameters changed
				if plan.ContextJSON.ValueString() == state.ContextJSON.ValueString() && 
				   plan.TemplateContent.ValueString() == state.TemplateContent.ValueString() &&
				   oldTargetPath == newTargetPath {
					shouldRegenerate = false
				}
			}
		}
	} else {
		// For inline templates, check if content has changed
		if plan.TemplateContent.ValueString() == state.TemplateContent.ValueString() &&
		   plan.ContextJSON.ValueString() == state.ContextJSON.ValueString() &&
		   oldTargetPath == newTargetPath {
			shouldRegenerate = false
		}
	}

	if shouldRegenerate {
		contextJSON := plan.ContextJSON.ValueString()
		generationResult, err := dagGenService.Generate(ctx, gcsPath, content, newTargetPath, contextJSON)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update DAG", err.Error())
			return
		}

		plan.ID = plan.TargetGCSPath
		plan.GeneratedFileChecksum = basetypes.NewStringValue(generationResult.Checksum)
		plan.GCSGenerationNumber = basetypes.NewStringValue(generationResult.Generation)

		// Store template checksum if using GCS template
		if gcsPath != "" {
			templateStatus, err := dagGenService.GetTemplateStatus(ctx, gcsPath)
			if err != nil {
				// Warning but don't fail
				resp.Diagnostics.AddWarning(
					"Could not get template status",
					fmt.Sprintf("Unable to get template status for %s: %v", gcsPath, err),
				)
				plan.TemplateChecksum = basetypes.NewStringValue("")
			} else {
				plan.TemplateChecksum = basetypes.NewStringValue(templateStatus.Checksum)
			}
		} else {
			plan.TemplateChecksum = basetypes.NewStringValue("")
		}
	} else {
		// No regeneration needed, just update the target path in state
		plan.ID = plan.TargetGCSPath
		plan.GeneratedFileChecksum = state.GeneratedFileChecksum
		plan.GCSGenerationNumber = state.GCSGenerationNumber
		plan.TemplateChecksum = state.TemplateChecksum
	}

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
