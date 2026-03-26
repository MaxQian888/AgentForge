DROP INDEX IF EXISTS idx_task_comments_task_created;
DROP INDEX IF EXISTS idx_entity_links_target;
DROP INDEX IF EXISTS idx_entity_links_source;

DROP TABLE IF EXISTS task_comments;
DROP TABLE IF EXISTS entity_links;

DROP TYPE IF EXISTS entity_link_type;
