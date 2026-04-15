# Automations API Notes

AgentForge project automations are event-condition-action rules configured through:

- `GET /api/v1/projects/:pid/automations`
- `POST /api/v1/projects/:pid/automations`
- `PUT /api/v1/projects/:pid/automations/:rid`
- `GET /api/v1/projects/:pid/automations/logs`

## Supported event types

- `task.status_changed`
- `task.assignee_changed`
- `task.field_changed`
- `task.due_date_approaching`
- `review.completed`
- `budget.threshold_reached`

## Supported action types

- `update_field`
- `assign_user`
- `send_notification`
- `move_to_column`
- `create_subtask`
- `send_im_message`
- `invoke_plugin`
- `start_workflow`

## Workflow orchestration

Use `start_workflow` when an automation rule is supposed to create a canonical workflow run. The action must provide a workflow plugin identifier:

```json
{
  "type": "start_workflow",
  "config": {
    "pluginId": "task-delivery-flow",
    "trigger": {
      "handoff": "due-date-escalation"
    }
  }
}
```

`start_workflow` uses the normal workflow runtime and persists a `WorkflowPluginRun`, so downstream operators can inspect the resulting run through existing workflow run surfaces.

## Generic plugin invocation

Use `invoke_plugin` only for generic integration plugin operations. It does not replace canonical workflow execution:

```json
{
  "type": "invoke_plugin",
  "config": {
    "pluginId": "slack-notifier",
    "operation": "send_message",
    "input": {
      "channel": "alerts"
    }
  }
}
```

If the intent is to orchestrate a workflow starter or workflow plugin, prefer `start_workflow` instead of `invoke_plugin`.
