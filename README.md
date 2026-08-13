# Terraform Provider Mirage

A Terraform provider for managing Airflow DAG generation through the Mirage ecosystem.

## Overview

The Mirage provider allows you to generate Airflow DAGs from Jinja2 templates and manage them in Google Cloud Storage. It integrates with a backend service to handle template rendering and file management.

## Requirements

- Terraform >= 1.0
- Go >= 1.24 (for development)
- Google Cloud Platform credentials (if using GCP authentication)

## Installation

### Using Terraform Registry

Add the following to your Terraform configuration:

```hcl
terraform {
  required_providers {
    mirage = {
      source  = "mm-aranda/mirage"
      version = "~> 0.0"
    }
  }
}

provider "mirage" {
  # Configuration options
}
```

### Local Development

For local development, build the provider and point Terraform at the build directory with a `dev_overrides` block in `~/.terraformrc`:

```bash
go build -o /path/to/tf-plugins/terraform-provider-mirage
```

```hcl
provider_installation {
  dev_overrides {
    "mm-aranda/mirage" = "/path/to/tf-plugins"
  }
  direct {}
}
```

With `dev_overrides` active, `terraform plan` and `apply` use the local build directly (no `terraform init` needed for this provider).

## Provider Configuration

The provider currently requires no configuration at the provider level. All configuration is done at the resource level.

```hcl
provider "mirage" {
  # No configuration required
}
```

## Resources

### `mirage_dag_generator`

Manages a generated file (typically an Airflow DAG) in Google Cloud Storage by processing Jinja2 templates.

#### Example Usage

##### Using a GCS Template

```hcl
resource "mirage_dag_generator" "example_dag" {
  dag_generator_backend_url = "https://your-backend-service.com"
  template_gcs_path        = "gs://your-bucket/templates/dag_template.py.j2"
  target_gcs_path          = "gs://your-bucket/dags/generated_dag.py"
  context_json             = jsonencode({
    dag_id = "example_dag"
    schedule_interval = "0 2 * * *"
    owner = "data-team"
  })
  use_gcp_service_account_auth = true
}
```

##### Using Inline Template Content

```hcl
resource "mirage_dag_generator" "inline_dag" {
  dag_generator_backend_url = "https://your-backend-service.com"
  template_content         = file("${path.module}/templates/dag_template.py.j2")
  target_gcs_path          = "gs://your-bucket/dags/inline_dag.py"
  context_json             = jsonencode({
    dag_id = "inline_dag"
    schedule_interval = "@daily"
  })
  use_gcp_service_account_auth = false
}
```

#### Argument Reference

- `dag_generator_backend_url` - (Required) The base URL of the backend service for DAG generation.
- `target_gcs_path` - (Required) The full `gs://` path for the generated output file.
- `template_gcs_path` - (Optional) The full `gs://` path to the source Jinja2 template. Mutually exclusive with `template_content`.
- `template_content` - (Optional) The content of the template as a string. Mutually exclusive with `template_gcs_path`.
- `context_json` - (Optional) A JSON string representing the dynamic context for template rendering.
- `use_gcp_service_account_auth` - (Optional) If true, authenticate requests to the backend with a Google OIDC ID token (service account, impersonated, or user credentials — see Authentication below). Default: `false`.

#### Attributes Reference

- `id` - The GCS path of the generated file.
- `generated_file_checksum` - The CRC32C checksum of the generated file in GCS.
- `gcs_generation_number` - The GCS generation number of the generated file.
- `template_checksum` - The CRC32C checksum of the template file in GCS (only populated when using `template_gcs_path`).

#### Import

DAG generator resources can be imported using the target GCS path:

```bash
terraform import mirage_dag_generator.example gs://your-bucket/dags/generated_dag.py
```

## Authentication

The provider supports two authentication methods:

### 1. Google Cloud Authentication

Set `use_gcp_service_account_auth = true` in your resource configuration. The backend is expected to be an IAM-protected Cloud Run service, which requires an OIDC ID token (plain OAuth2 access tokens are rejected). The provider obtains one from the first of these that applies:

1. **Impersonation** — if the `GOOGLE_IMPERSONATE_SERVICE_ACCOUNT` environment variable is set, the provider mints an ID token as that service account. The caller needs `roles/iam.serviceAccountTokenCreator` on it.
2. **Service account credentials** — if Application Default Credentials (ADC) resolve to a service account (CI, Cloud Run, GCE), the standard ID token flow is used.
3. **User credentials** — if ADC resolve to a user account (`gcloud auth application-default login`), the provider extracts the `id_token` returned alongside the access token, the same mechanism `gcloud auth print-identity-token` uses. Cloud Run accepts these user ID tokens.

In every case the caller needs `roles/run.invoker` on the backend Cloud Run service. If no ID token can be obtained, requests fail with an error explaining what to configure.

### 2. No Authentication

Set `use_gcp_service_account_auth = false` or omit the attribute. Requests will be sent without authentication headers.

## Backend Service Requirements

The Mirage provider expects a backend service with the following endpoints:

### POST `/generate`

Generate or update a DAG file.

**Request Body:**
```json
{
  "template_gcs_path": "gs://bucket/template.j2",
  "template_content": "template content string",
  "target_gcs_path": "gs://bucket/output.py",
  "context_json": "{\"key\": \"value\"}"
}
```

**Response:**
```json
{
  "checksum": "abc123",
  "generation": "1234567890"
}
```

### GET `/status`

Get the current status of a generated file.

**Query Parameters:**
- `target_gcs_path` - The GCS path of the file

**Response:**
```json
{
  "checksum": "abc123",
  "generation": "1234567890"
}
```

### GET `/template-status`

Get the current status of a template file.

**Query Parameters:**
- `template_gcs_path` - The GCS path of the template file

**Response:**
```json
{
  "checksum": "abc123",
  "generation": "1234567890",
  "last_modified": "2024-01-01T12:00:00Z",
  "exists": true
}
```

### POST `/delete`

Delete a generated file.

**Request Body:**
```json
{
  "target_gcs_path": "gs://bucket/file.py"
}
```

## Development

### Building the Provider

```bash
go build -o terraform-provider-mirage
```

### Running Tests

```bash
go test ./...
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the terms specified in the LICENSE file.

## Support

For issues and questions, please open an issue in the GitHub repository. 