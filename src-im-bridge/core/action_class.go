package core

// ActionClassForCommand maps a bridge slash command to its blast-radius
// classification for rate-limit and audit bucketing. Unknown or empty
// commands default to `read`, the most conservative bucket. The mapping
// is intentionally coarse (top-level command only); per-subcommand
// classification can be refined later by the command handlers themselves
// via re-scoped rate checks.
func ActionClassForCommand(cmd string) RateActionClass {
	switch cmd {
	case "":
		return ActionClassRead
	// Reads
	case "/help", "/queue", "/team", "/memory", "/project", "/login", "/cost", "/document":
		return ActionClassRead
	// Writes
	case "/task", "/agent", "/review", "/sprint":
		return ActionClassWrite
	// Destructive (blast radius: side-effects on runtime, plugins, agents)
	case "/tools":
		return ActionClassDestructive
	default:
		return ActionClassRead
	}
}
