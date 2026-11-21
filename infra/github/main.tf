resource "github_repository" "learnings" {
  allow_auto_merge            = false
  allow_merge_commit          = false
  allow_rebase_merge          = false
  allow_squash_merge          = true
  allow_update_branch         = true
  archive_on_destroy          = true
  delete_branch_on_merge      = true
  description                 = null
  has_discussions             = false
  has_issues                  = false
  has_projects                = false
  has_wiki                    = false
  name                        = "learnings"
  squash_merge_commit_message = "COMMIT_MESSAGES"
  squash_merge_commit_title   = "COMMIT_OR_PR_TITLE"
  topics                      = []
  visibility                  = "public"
  lifecycle {
    prevent_destroy = true
  }
}

resource "github_repository_ruleset" "main" {
  name        = "main"
  target      = "branch"
  repository  = github_repository.learnings.name
  enforcement = "active"
  conditions {
    ref_name {
      include = ["refs/heads/main"]
      exclude = []
    }
  }
  bypass_actors {
    actor_type  = "RepositoryRole"
    actor_id    = 5 # GitHub's fixed ID for the Admin role
    bypass_mode = "always"
  }
  rules {
    deletion         = true
    non_fast_forward = true
    required_status_checks {
      strict_required_status_checks_policy = true
      dynamic "required_check" {
        for_each = ["checks", "lint", "test"]
        content {
          context        = required_check.value
          integration_id = 15368 # GitHub Actions app ID
        }
      }
    }
  }
}
