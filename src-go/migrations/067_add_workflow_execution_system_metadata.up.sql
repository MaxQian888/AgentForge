-- system_metadata is a backend-only jsonb document attached to every workflow
-- execution. Reserved system keys (per spec §6.3):
--   reply_target          {provider, chat_id, thread_id, message_id, tenant_id}
--   im_dispatched         bool   ← outbound_dispatcher reads this; im_send node sets it true
--   final_output          jsonb  ← optional author-declared completion summary
--
-- DAG node code MUST NOT read or write this column directly; it is owned by
-- trigger_handler (writes reply_target on execution create), the im_send
-- node (sets im_dispatched), and the outbound_dispatcher (reads both).
-- Author-facing data lives in data_store.
ALTER TABLE workflow_executions
    ADD COLUMN system_metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
