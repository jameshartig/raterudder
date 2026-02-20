resource "google_cloud_scheduler_job" "raterudder_update_sites" {
  name        = "raterudder-update-sites"
  description = "Triggers the /api/updateSites endpoint every 20 minutes"
  schedule    = "*/20 * * * *"
  time_zone   = "America/Chicago"
  region      = "us-central1"
  project     = var.project_id
  paused      = !var.schedule_enabled
  # this just needs to be larger than the run service timeout
  attempt_deadline = "90s"

  http_target {
    http_method = "POST"
    uri         = "${local.run_deterministic_uri}/api/updateSites"

    oidc_token {
      service_account_email = google_service_account.raterudder.email
      audience              = local.run_deterministic_uri
    }
  }
}
