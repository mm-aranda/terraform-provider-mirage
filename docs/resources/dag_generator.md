# mirage_dag_generator Resource

Manages a generated file (typically an Airflow DAG) in Google Cloud Storage by processing Jinja2 templates through a backend service.

## Example Usage

### Using a GCS Template

```terraform
resource "mirage_dag_generator" "example_dag" {
  dag_generator_backend_url = "https://your-backend-service.com"
  template_gcs_path        = "gs://your-bucket/templates/dag_template.py.j2"
  target_gcs_path          = "gs://your-bucket/dags/generated_dag.py"
  context_json             = jsonencode({
    dag_id = "example_dag"
    schedule_interval = "0 2 * * *"
    owner = "data-team"
    retries = 3
  })
  use_gcp_service_account_auth = true
}
```

### Using Inline Template Content

```terraform
resource "mirage_dag_generator" "inline_dag" {
  dag_generator_backend_url = "https://your-backend-service.com"
  template_content         = file("${path.module}/templates/dag_template.py.j2")
  target_gcs_path          = "gs://your-bucket/dags/inline_dag.py"
  context_json             = jsonencode({
    dag_id = "inline_dag"
    schedule_interval = "@daily"
    start_date = "2024-01-01"
  })
  use_gcp_service_account_auth = false
}
```

### Complex Context Example

```terraform
resource "mirage_dag_generator" "complex_dag" {
  dag_generator_backend_url = "https://dag-generator.example.com"
  template_gcs_path        = "gs://my-templates/complex_dag.py.j2"
  target_gcs_path          = "gs://my-dags/complex_dag.py"
  context_json             = jsonencode({
    dag_id = "complex_processing_dag"
    schedule_interval = "0 */6 * * *"
    owner = "analytics-team"
    email = ["team@company.com"]
    depends_on_past = false
    catchup = false
    max_active_runs = 1
    tasks = [
      {
        task_id = "extract_data"
        command = "python /scripts/extract.py"
        pool = "default_pool"
      },
      {
        task_id = "transform_data"
        command = "python /scripts/transform.py"
        pool = "heavy_pool"
      },
      {
        task_id = "load_data"
        command = "python /scripts/load.py"
        pool = "default_pool"
      }
    ]
  })
  use_gcp_service_account_auth = true
}
```

## Argument Reference

The following arguments are supported:

* `dag_generator_backend_url` - (Required) The base URL of the backend service for DAG generation.
* `target_gcs_path` - (Required) The full `gs://` path for the generated output file.
* `template_gcs_path` - (Optional) The full `gs://` path to the source Jinja2 template. Mutually exclusive with `template_content`.
* `template_content` - (Optional) The content of the template as a string. Mutually exclusive with `template_gcs_path`.
* `context_json` - (Optional) A JSON string representing the dynamic context for template rendering.
* `use_gcp_service_account_auth` - (Optional) If true, authenticate requests using the machine's GCP service account. Defaults to `false`.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The GCS path of the generated file (same as `target_gcs_path`).
* `generated_file_checksum` - The CRC32C checksum of the generated file in GCS.
* `gcs_generation_number` - The GCS generation number of the generated file.
* `template_checksum` - The CRC32C checksum of the template file in GCS (only populated when using `template_gcs_path`).

## Import

DAG generator resources can be imported using the target GCS path:

```bash
terraform import mirage_dag_generator.example gs://your-bucket/dags/generated_dag.py
```

## Notes

### Template Source Requirements

You must specify exactly one of `template_gcs_path` or `template_content`. The resource will fail if both are specified or if neither is specified.

### Authentication

When `use_gcp_service_account_auth` is set to `true`, the provider will:
1. First attempt to use ID tokens for service account authentication
2. Fall back to OAuth2 access tokens if user credentials are detected
3. Log authentication method selection for debugging

### File Management

The resource tracks the generated file using:
- **Checksum**: CRC32C checksum for content verification
- **Generation Number**: GCS generation number for versioning

These values are updated on each successful generation and can be used for drift detection.

### Backend Service Integration

This resource requires a compatible backend service with the following endpoints:
- `POST /generate` - Generate or update a file
- `GET /status` - Get file status and metadata
- `POST /delete` - Delete a file
- `GET /template-status` - Get template file status and metadata

### Automatic File Management

The resource includes several automatic behaviors to ensure proper file management:

#### Target Path Changes

When `target_gcs_path` is changed, the resource will:
1. Delete the old file at the previous target path
2. Generate a new file at the new target path
3. Update the resource state with the new path

If deletion of the old file fails, a warning will be logged but the operation will continue.

#### Template Change Detection

When using `template_gcs_path`, the resource automatically detects if the remote template file has been modified:
1. The resource tracks the template's checksum in its state
2. On updates, it compares the current template checksum with the stored checksum
3. If the template has changed, the file is automatically regenerated
4. If the template hasn't changed and no other parameters have changed, regeneration is skipped for efficiency

This ensures that generated files are always up-to-date with their templates without unnecessary regeneration.

See the main provider documentation for detailed API specifications. 