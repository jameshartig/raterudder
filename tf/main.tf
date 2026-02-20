terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.17.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "7.17.0"
    }
  }
}

provider "google" {
  project = var.project_id
}


variable "project_id" {
  description = "gcp project ID"
  default     = "raterudder"
}

data "google_project" "raterudder" {
  project_id = var.project_id
}

variable "schedule_enabled" {
  description = "Whether the scheduler is enabled"
  type        = bool
  default     = true
}

variable "join_form_url" {
  description = "The URL for the Google Form to join the beta"
  type        = string
  default     = "https://forms.gle/Z6dJGBebud9J32vZA"
}


variable "networks" {
  default = {
    "us-central1" : "10.0.0.0/24",
  }
}

module "enabled_google_apis" {
  source  = "terraform-google-modules/project-factory/google//modules/project_services"
  version = "~> 18.1"

  project_id                  = var.project_id
  disable_services_on_destroy = false

  activate_apis = [
    "run.googleapis.com",
    "iam.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "cloudtrace.googleapis.com",
    "cloudbuild.googleapis.com",
    "storage.googleapis.com",
    "containerregistry.googleapis.com",
    "containeranalysis.googleapis.com",
    "artifactregistry.googleapis.com",
    "compute.googleapis.com",
    "developerconnect.googleapis.com",
    "cloudscheduler.googleapis.com",
    "firestore.googleapis.com",
    "secretmanager.googleapis.com",
  ]
}
