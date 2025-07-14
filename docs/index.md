# Mirage Provider

The Mirage provider is used to manage Airflow DAG generation through the Mirage ecosystem. It allows you to generate DAGs from Jinja2 templates and store them in Google Cloud Storage.

## Example Usage

```terraform
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

resource "mirage_dag_generator" "example" {
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

## Schema

The provider schema is currently empty as all configuration is done at the resource level. 