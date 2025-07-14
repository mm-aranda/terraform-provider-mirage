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
      source  = "localhost/my-org/mirage"
      version = "~> 1.0"
    }
  }
}

provider "mirage" {
  # Configuration options
}
```

### Local Development

For local development, build and install the provider:

```bash
go build -o terraform-provider-mirage
mkdir -p ~/.terraform.d/plugins/localhost/my-org/mirage/1.0.0/darwin_amd64
cp terraform-provider-mirage ~/.terraform.d/plugins/localhost/my-org/mirage/1.0.0/darwin_amd64/
```

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
- `use_gcp_service_account_auth` - (Optional) If true, authenticate requests using the machine's GCP service account. Default: `false`.

#### Attributes Reference

- `id` - The GCS path of the generated file.
- `generated_file_checksum` - The CRC32C checksum of the generated file in GCS.
- `gcs_generation_number` - The GCS generation number of the generated file.

#### Import

DAG generator resources can be imported using the target GCS path:

```bash
terraform import mirage_dag_generator.example gs://your-bucket/dags/generated_dag.py
```

## Authentication

The provider supports two authentication methods:

### 1. GCP Service Account Authentication

Set `use_gcp_service_account_auth = true` in your resource configuration. The provider will use:
- ID tokens for service account authentication
- OAuth2 access tokens as fallback for user credentials

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