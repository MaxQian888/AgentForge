package service

import (
	"encoding/json"
	"fmt"
)

var supportedAutomationActionTypes = map[string]struct{}{
	"update_field":      {},
	"assign_user":       {},
	"send_notification": {},
	"move_to_column":    {},
	"create_subtask":    {},
	"send_im_message":   {},
	"invoke_plugin":     {},
	"start_workflow":    {},
}

func ValidateAutomationActions(raw json.RawMessage) error {
	actions, err := decodeAutomationActions(string(raw))
	if err != nil {
		return err
	}
	for index, action := range actions {
		if _, ok := supportedAutomationActionTypes[action.Type]; !ok {
			return fmt.Errorf("unsupported automation action %q", action.Type)
		}
		if action.Type != "start_workflow" {
			continue
		}
		if stringValue(action.Config["pluginId"]) == "" {
			return fmt.Errorf("automation action %d start_workflow requires pluginId", index)
		}
		if rawTrigger, ok := action.Config["trigger"]; ok && rawTrigger != nil {
			if _, ok := rawTrigger.(map[string]any); !ok {
				return fmt.Errorf("automation action %d start_workflow trigger must be an object", index)
			}
		}
	}
	return nil
}
