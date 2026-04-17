## Context

The AgentForge frontend currently provides basic dashboard functionality with metric cards, activity feeds, and configuration panels. The existing implementation uses Next.js 16 with React 19, shadcn/ui components, Tailwind CSS v4, and Zustand for state management. The current sidebar navigation organizes features into Workspace, Project, Operations, and Configuration groups.

**Current Limitations:**
- Dashboard widgets are static with no interactivity
- Task management lacks visual board views (kanban, timeline)
- Agent lifecycle operations are scattered across multiple pages
- No visual feedback for IM bridge health or scheduler job status
- Plugin management requires manual configuration files
- No global quick navigation (command palette)
- Limited responsive design for different screen sizes

**Stakeholders:**
- End users (developers using the platform)
- Platform operators (monitoring system health)
- Plugin developers (managing plugin lifecycle)

## Goals / Non-Goals

**Goals:**
- Provide comprehensive visual dashboards for all major features
- Enable full agent lifecycle management from a single panel
- Implement multi-view task visualization (kanban, timeline, calendar)
- Add real-time status indicators for system health
- Create visual workflow builder for automation
- Implement global command palette for quick navigation
- Ensure responsive layouts across all screen sizes

**Non-Goals:**
- Mobile-first design (desktop is primary use case)
- Offline support (requires network connectivity)
- Real-time collaboration features (multi-user editing)
- Custom theming beyond light/dark mode
- Accessibility beyond WCAG 2.1 AA compliance

## Decisions

### 1. Chart Library Selection

**Decision:** Use Recharts for data visualizations

**Rationale:**
- Built on React primitives, integrates well with shadcn/ui
- Declarative API matches React patterns
- Good TypeScript support
- Responsive by default
- Active maintenance and community

**Alternatives Considered:**
- Chart.js: More imperative, requires react-chartjs-2 wrapper
- D3.js: Too low-level, steeper learning curve
- Victory: Smaller ecosystem, less documentation

### 2. Drag-and-Drop for Workflow Builder

**Decision:** Use @dnd-kit/core for drag-and-drop functionality

**Rationale:**
- Modern, accessible drag-and-drop library
- Built for React with hooks API
- Supports complex use cases (multi-container, collision detection)
- No additional dependencies
- Active development and good documentation

**Alternatives Considered:**
- react-beautiful-dnd: Deprecated, no active maintenance
- react-dnd: Requires HTML5 drag-and-drop knowledge
- dnd.js: Less React-native, more configuration

### 3. State Management for Complex Panels

**Decision:** Extend Zustand stores with derived selectors

**Rationale:**
- Already in use, maintains consistency
- Selectors prevent unnecessary re-renders
- Works well with React 19 concurrent features
- Simple debugging with devtools

**Alternatives Considered:**
- React Query for server state: Adds complexity for already-fetched data
- Jotai: New paradigm, migration cost

### 4. Command Palette Implementation

**Decision:** Use cmdk library for command palette

**Rationale:**
- Built by Radix UI team (same as shadcn/ui primitives)
- Accessible by default
- Supports keyboard navigation and filtering
- Easy integration with existing UI patterns

**Alternatives Considered:**
- Custom implementation: Higher maintenance burden
- kbar: Less customizable, smaller community

### 5. Real-time Updates Strategy

**Decision:** Polling with configurable intervals (5-30 seconds)

**Rationale:**
- Simpler than WebSocket implementation
- Works with existing REST API
- Configurable per-component for performance
- Graceful degradation on network issues

**Alternatives Considered:**
- WebSockets: Requires backend changes, connection management
- Server-Sent Events: One-way only, still needs backend changes
- SWR/React Query polling: Adds dependency for limited benefit

### 6. Component Architecture

**Decision:** Feature-based folder structure with shared UI components

**Rationale:**
- Each feature panel is self-contained
- Shared components in `components/ui/` and `components/shared/`
- Feature-specific components in `components/<feature>/`
- Easier to maintain and test in isolation

**Structure:**
```
components/
├── ui/                    # shadcn/ui primitives
├── shared/               # Reusable shared components
├── dashboard/            # Dashboard-specific widgets
├── agents/               # Agent workspace components
├── tasks/                # Task board components
├── reviews/              # Review pipeline components
├── cost/                 # Cost dashboard components
├── im/                   # IM bridge components
├── plugins/              # Plugin marketplace components
├── workflow/             # Workflow builder components
├── memory/               # Memory explorer components
└── scheduler/            # Scheduler panel components
```

## Risks / Trade-offs

### Performance Risk: Large Data Sets

**Risk:** Kanban board and activity feeds may have hundreds of items, causing render performance issues.

**Mitigation:**
- Use virtual scrolling (react-window or similar) for lists > 100 items
- Implement pagination for activity feeds
- Debounce search/filter inputs
- Memoize expensive computations

### Complexity Risk: Workflow Builder

**Risk:** Visual workflow builder is complex and may have many edge cases.

**Mitigation:**
- Start with simple node types (trigger, action, condition)
- Validate connections client-side before saving
- Provide workflow templates for common patterns
- Add undo/redo support from the start

### Bundle Size Risk: Additional Dependencies

**Risk:** New libraries (recharts, @dnd-kit, cmdk) increase bundle size.

**Mitigation:**
- Use dynamic imports for heavy components (workflow builder, charts)
- Tree-shake unused chart types
- Monitor bundle size with CI checks
- Target < 500KB initial bundle

### API Dependency Risk

**Risk:** New panels require new backend APIs that may not exist.

**Mitigation:**
- Document all required API endpoints in specs
- Use mock data for initial development
- Coordinate with backend team on API priorities
- Graceful fallback when APIs unavailable

### Learning Curve Risk

**Risk:** Users may find new features overwhelming.

**Mitigation:**
- Preserve existing navigation patterns
- Add onboarding tooltips for new features
- Provide video tutorials for complex features (workflow builder)
- Keep simple mode available alongside advanced features

## Migration Plan

### Phase 1: Foundation (Week 1-2)
1. Add chart library and command palette dependencies
2. Create responsive layout system
3. Implement base widget components with loading states
4. Add command palette with navigation actions

### Phase 2: Dashboard Enhancement (Week 2-3)
1. Replace metric cards with interactive widgets
2. Add real-time status indicators
3. Implement activity feed filtering
4. Add dashboard layout preferences

### Phase 3: Feature Panels (Week 3-6)
1. Agent workspace panel
2. Task multi-view board
3. Review pipeline visualization
4. Cost dashboard charts
5. IM bridge status panel
6. Scheduler control panel

### Phase 4: Advanced Features (Week 6-8)
1. Plugin marketplace panel
2. Workflow visual builder
3. Memory explorer panel

### Rollback Strategy
- All new panels are behind feature flags
- Existing pages remain accessible during migration
- Users can opt into new dashboard layout
- Per-feature rollback via feature flags

## Open Questions

1. **Workflow Node Types:** What specific node types should the workflow builder support initially?
   - Proposed: Trigger (webhook, schedule), Action (HTTP request, script), Condition (if/else)
   - Need stakeholder input on priority

2. **Cost Granularity:** Should cost charts show per-agent breakdown or aggregate only?
   - Aggregate is simpler, per-agent requires new API
   - Performance impact of detailed data

3. **Memory Explorer Scope:** Should users be able to edit/delete stored memories?
   - Read-only is safer, editable adds complexity
   - Security implications of memory modification

4. **Plugin Marketplace Source:** Curated plugins only or community submissions?
   - Curated ensures quality but limits growth
   - Community needs moderation system
