package core

import (
	"context"
	"strings"
	"time"
)

type BridgeCapability string

const (
	BridgeCapabilityDecompose      BridgeCapability = "decompose"
	BridgeCapabilityGenerate       BridgeCapability = "generate"
	BridgeCapabilityClassifyIntent BridgeCapability = "classify-intent"
	BridgeCapabilityPool           BridgeCapability = "pool"
	BridgeCapabilityHealth         BridgeCapability = "health"
	BridgeCapabilityRuntimes       BridgeCapability = "runtimes"
	BridgeCapabilityTools          BridgeCapability = "tools"
)

type CommandRouteTarget string

const (
	CommandRouteGoAPI           CommandRouteTarget = "go_api"
	CommandRouteBridgeOnly      CommandRouteTarget = "bridge_only"
	CommandRouteBridgePreferred CommandRouteTarget = "bridge_preferred"
)

type CommandRoute struct {
	Command       string
	Subcommand    string
	Capability    BridgeCapability
	Target        CommandRouteTarget
	AllowFallback bool
}

type BridgeCapabilityProbe interface {
	Check(ctx context.Context, capability BridgeCapability) error
}

type BridgeCapabilityProbeFunc func(ctx context.Context, capability BridgeCapability) error

func (f BridgeCapabilityProbeFunc) Check(ctx context.Context, capability BridgeCapability) error {
	return f(ctx, capability)
}

type bridgeCapabilityCacheEntry struct {
	checkedAt time.Time
	err       error
}

var defaultCommandRoutes = map[string]CommandRoute{
	commandRouteKey("/task", "decompose"):   {Command: "/task", Subcommand: "decompose", Capability: BridgeCapabilityDecompose, Target: CommandRouteBridgePreferred, AllowFallback: true},
	commandRouteKey("/task ai", "generate"): {Command: "/task ai", Subcommand: "generate", Capability: BridgeCapabilityGenerate, Target: CommandRouteBridgeOnly},
	commandRouteKey("/task ai", "classify"): {Command: "/task ai", Subcommand: "classify", Capability: BridgeCapabilityClassifyIntent, Target: CommandRouteBridgeOnly},
	commandRouteKey("/agent", "status"):     {Command: "/agent", Subcommand: "status", Capability: BridgeCapabilityPool, Target: CommandRouteBridgePreferred, AllowFallback: true},
	commandRouteKey("/agent", "runtimes"):   {Command: "/agent", Subcommand: "runtimes", Capability: BridgeCapabilityRuntimes, Target: CommandRouteBridgeOnly},
	commandRouteKey("/agent", "health"):     {Command: "/agent", Subcommand: "health", Capability: BridgeCapabilityHealth, Target: CommandRouteBridgeOnly},
	commandRouteKey("/tools", "list"):       {Command: "/tools", Subcommand: "list", Capability: BridgeCapabilityTools, Target: CommandRouteBridgeOnly},
	commandRouteKey("/tools", "install"):    {Command: "/tools", Subcommand: "install", Capability: BridgeCapabilityTools, Target: CommandRouteBridgeOnly},
	commandRouteKey("/tools", "uninstall"):  {Command: "/tools", Subcommand: "uninstall", Capability: BridgeCapabilityTools, Target: CommandRouteBridgeOnly},
	commandRouteKey("/tools", "restart"):    {Command: "/tools", Subcommand: "restart", Capability: BridgeCapabilityTools, Target: CommandRouteBridgeOnly},
}

func commandRouteKey(command, subcommand string) string {
	return strings.TrimSpace(command) + "::" + strings.TrimSpace(subcommand)
}

func resolveCommandRoute(command, subcommand string) CommandRoute {
	if route, ok := defaultCommandRoutes[commandRouteKey(command, subcommand)]; ok {
		return route
	}
	return CommandRoute{
		Command:    strings.TrimSpace(command),
		Subcommand: strings.TrimSpace(subcommand),
		Target:     CommandRouteGoAPI,
	}
}
