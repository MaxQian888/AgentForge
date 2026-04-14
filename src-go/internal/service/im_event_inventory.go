package service

var canonicalIMEventInventory = []string{
	"task.created",
	"task.completed",
	"review.completed",
	"agent.started",
	"agent.completed",
	"budget.warning",
	"sprint.started",
	"sprint.completed",
	"review.requested",
	"wiki.page.updated",
	"wiki.version.published",
	"wiki.comment.mention",
	"workflow.failed",
}

func CanonicalIMEventInventory() []string {
	return append([]string(nil), canonicalIMEventInventory...)
}

