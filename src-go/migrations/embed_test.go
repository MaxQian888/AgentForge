package migrations

import (
	"io/fs"
	"testing"
)

func TestEmbeddedMigrationsKeepReleasedFilenamesStable(t *testing.T) {
	expected := []string{
		"019_create_scheduler_control_plane_tables.up.sql",
		"019_create_scheduler_control_plane_tables.down.sql",
		"020_create_agent_pool_queue_entries.up.sql",
		"020_create_agent_pool_queue_entries.down.sql",
		"021_create_agent_events.up.sql",
		"021_create_agent_events.down.sql",
		"022_create_custom_field_defs.up.sql",
		"022_create_custom_field_defs.down.sql",
		"023_create_custom_field_values.up.sql",
		"023_create_custom_field_values.down.sql",
		"024_create_saved_views.up.sql",
		"024_create_saved_views.down.sql",
		"025_create_form_definitions.up.sql",
		"025_create_form_definitions.down.sql",
		"026_create_form_submissions.up.sql",
		"026_create_form_submissions.down.sql",
		"027_create_automation_rules.up.sql",
		"027_create_automation_rules.down.sql",
		"028_create_automation_logs.up.sql",
		"028_create_automation_logs.down.sql",
		"029_create_dashboard_configs.up.sql",
		"029_create_dashboard_configs.down.sql",
		"030_create_dashboard_widgets.up.sql",
		"030_create_dashboard_widgets.down.sql",
		"031_create_milestones.up.sql",
		"031_create_milestones.down.sql",
		"032_add_milestone_ids_to_tasks_and_sprints.up.sql",
		"032_add_milestone_ids_to_tasks_and_sprints.down.sql",
		"033_add_agent_pool_queue_priority.up.sql",
		"033_add_agent_pool_queue_priority.down.sql",
		"034_create_dispatch_attempts.up.sql",
		"034_create_dispatch_attempts.down.sql",
		"035_create_plugin_control_plane_tables.up.sql",
		"035_create_plugin_control_plane_tables.down.sql",
		"036_create_plugin_persistence_tables.up.sql",
		"036_create_plugin_persistence_tables.down.sql",
		"037_add_review_execution_metadata.up.sql",
		"037_add_review_execution_metadata.down.sql",
		"038_create_entity_links_and_task_comments.up.sql",
		"038_create_entity_links_and_task_comments.down.sql",
		"039_create_wiki_workspace_tables.up.sql",
		"039_create_wiki_workspace_tables.down.sql",
		"040_add_review_pending_human_status.up.sql",
		"040_add_review_pending_human_status.down.sql",
		"041_align_member_contract_with_documented_status_and_im_identity.up.sql",
		"041_align_member_contract_with_documented_status_and_im_identity.down.sql",
		"042_add_agent_run_cost_accounting.up.sql",
		"042_add_agent_run_cost_accounting.down.sql",
	}

	for _, name := range expected {
		if _, err := fs.Stat(FS, name); err != nil {
			t.Fatalf("expected released migration %q to remain embedded: %v", name, err)
		}
	}
}
