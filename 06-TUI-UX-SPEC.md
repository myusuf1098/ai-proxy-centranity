# ProxyGateway Enterprise — TUI UX Specification

**Document ID:** PG-UX-006  
**Version:** 2.0  
**Status:** Approved Design Baseline  
**Related Documents:** `01-PRD.md`, `02-ROADMAP.md`, `03-ARCHITECTURE.md`, `04-TECH-STACK.md`, `05-REFACTORY-PLAN.md`, `07-IMPLEMENTATION-DOCUMENTATION.md`

---

## 1. Purpose

This document defines the visual language, interaction model, information architecture, terminal layouts, accessibility behavior, and UI implementation rules for the ProxyGateway Enterprise TUI.

The TUI is an **administrative client of the ProxyGateway Management API**. It MUST NOT access PostgreSQL, Redis, or 9Router storage directly.

The TUI is designed for operators who need to:

- monitor live traffic;
- inspect models/providers;
- switch routing;
- manage API keys and policies;
- configure proxy profiles;
- inspect quotas, limits and budgets;
- review audit records;
- execute safe operational actions.

---

## 2. Design Principles

### 2.1 Calm Operations Console

The interface must look professional and operational rather than promotional.

Required characteristics:

- restrained contrast;
- low visual noise;
- consistent spacing;
- compact but readable information density;
- clear hierarchy;
- subtle status emphasis;
- no excessive animation;
- no decorative gradients;
- no 3D icons;
- no glossy or neon treatment.

### 2.2 Minimal Iconography

Use simple text symbols or monochrome glyphs.

Preferred symbols:

```text
●  active/healthy
○  inactive
△  warning
×  error
→  route/action
◆  selected
─  separator
```

Do not use:

- 3D icons;
- emoji as primary UI controls;
- oversized decorative Unicode art;
- icon-only destructive actions.

Every icon/symbol MUST have an adjacent textual meaning when ambiguity is possible.

### 2.3 Visual Continuity

The color system is semantic and reused across every screen.

Recommended dark-terminal palette:

```text
Background        #0F1115
Surface           #151922
Surface Alt       #1B2029
Border            #2B313C
Primary Text      #D9DEE7
Muted Text        #8992A3
Primary Accent    #7F9DBB
Success           #7FA88A
Warning           #B9A06A
Error             #B87979
Info              #7896B5
```

The exact terminal rendering may vary by theme, but the semantic roles MUST remain consistent.

Avoid highly saturated colors.

### 2.4 Information Hierarchy

Priority:

```text
1. System state
2. Current operation
3. Health
4. Business/usage values
5. Secondary metadata
6. Actions
```

---

## 3. Terminal Targets

### Minimum Supported

```text
80 x 24
```

### Recommended

```text
120 x 30
```

### Optimal

```text
160 x 40+
```

The UI MUST degrade gracefully:

- hide secondary columns first;
- abbreviate long identifiers;
- collapse low-priority panels;
- never truncate actionable status without an accessible detail view.

---

## 4. Global Layout

All pages use a consistent three-zone structure.

```text
┌──────────────────────────────────────────────────────────────────────┐
│ HEADER                                                               │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│ MAIN CONTENT                                                         │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│ FOOTER / SHORTCUTS                                                   │
└──────────────────────────────────────────────────────────────────────┘
```

### Header

Contains:

```text
ProxyGateway
environment
version
online/degraded/offline state
uptime
```

Example:

```text
PROXYGATEWAY   production   ● ONLINE   v1.0.0   uptime 04:32:18
```

### Footer

Displays contextual commands:

```text
↑↓ Navigate   Enter Open   E Edit   S Switch   / Search   ? Help   Q Quit
```

---

## 5. Main Navigation

```text
1  Dashboard
2  Requests
3  Models
4  Providers
5  API Keys
6  Policies
7  Routing
8  Proxies
9  Usage
0  Audit
S  Settings
?  Help
Q  Quit
```

Optional command palette:

```text
:
```

Examples:

```text
:model switch coding
:provider test anthropic
:proxy test SOCKS5-01
:policy edit homepc
```

---

## 6. Dashboard

### Purpose

Fast operational overview.

### Layout

```text
┌──────────────────────────────────────────────────────────────────────┐
│ PROXYGATEWAY   production   ● ONLINE   v1.0.0                       │
├──────────────────────────────────────────────────────────────────────┤
│ REQUESTS       SUCCESS        TOKENS         P95 LATENCY              │
│ 12,482         99.42%         8.74M          382 ms                  │
│                                                                      │
├────────────────────────────────────┬─────────────────────────────────┤
│ LIVE REQUESTS                      │ PROVIDER HEALTH                 │
│ 12:42:18 coding      DeepSeek  ✓   │ ● OpenCode      99.8%   312 ms │
│ 12:42:17 reasoning   Claude    ✓   │ ● Anthropic     99.4%   421 ms │
│ 12:42:16 fast        Google   △   │ △ Google        98.1%   389 ms │
├────────────────────────────────────┼─────────────────────────────────┤
│ MODEL USAGE                        │ ROUTING                         │
│ deepseek-v4   ███████████ 62%     │ coding                         │
│ mimo-v2.5     █████       27%      │ → deepseek-v4    primary       │
│ claude        ██           8%      │ → mimo-v2.5      fallback      │
│ gemini        █             3%     │ → google         fallback      │
└────────────────────────────────────┴─────────────────────────────────┘
```

### Dashboard Rules

- No more than four high-level KPI cards.
- Use bars/sparklines only where they aid comparison.
- Status colors are semantic, not decorative.
- The dashboard MUST remain readable without color.

---

## 7. Requests Screen

```text
┌─ REQUESTS ────────────────────────────────────────────────────────────┐
│ Filter: all          Search: __________________________________       │
├──────┬────────────┬────────────────────┬─────────┬────────┬──────────┤
│ STAT │ TIME       │ MODEL              │ PROVIDER│ LATENCY│ RESULT   │
├──────┼────────────┼────────────────────┼─────────┼────────┼──────────┤
│ ●    │ 12:42:18   │ deepseek-v4-flash  │ OpenCode│ 312 ms │ 200      │
│ ●    │ 12:42:17   │ mimo-v2.5-free     │ OpenCode│ 348 ms │ 200      │
│ △    │ 12:42:16   │ gemini-flash       │ Google  │ 1.2 s  │ 429      │
└──────┴────────────┴────────────────────┴─────────┴────────┴──────────┘
```

Request detail:

```text
Request ID
API key identity
Model requested
Model resolved
Provider
Routing reason
Start/end
Latency
Input/output tokens
Retry count
Circuit state
Final status
```

Prompt/response content MUST NOT be shown by default.

---

## 8. Models Screen

Actions:

```text
A Add
E Edit
S Switch
T Test
Space Enable/Disable
D Disable/Delete
```

Detail fields:

- model ID;
- aliases;
- provider;
- enabled;
- priority;
- timeout;
- fallback;
- capabilities;
- cost metadata;
- policy references.

### Model Switch Flow

```text
Select model
   |
Confirm
   |
Preview affected routes
   |
Apply
   |
Audit
   |
Result
```

---

## 9. Providers Screen

Provider list shows:

```text
status
name
type
health
latency
request rate
error rate
proxy profile
priority
```

Provider detail actions:

```text
E Edit
T Test
P Proxy
R Routes
D Disable
```

Provider mutation must show an impact preview where practical.

---

## 10. API Keys Screen

Displayed columns:

```text
Name
Prefix
Status
Policy
Usage
Expires
```

Never display the complete key after creation.

Create flow:

```text
Name
Policy
Model allowlist
Provider allowlist
Rate limits
Quota
Budget
Expiration
Confirm
```

The secret is shown once and requires explicit acknowledgement.

---

## 11. Policies Screen

Policy categories:

```text
AUTH
MODEL
PROVIDER
RATE LIMIT
QUOTA
BUDGET
ROUTING
PROXY
NETWORK
```

Policy editor MUST provide:

- current value;
- proposed value;
- validation state;
- conflict warning;
- precedence explanation.

---

## 12. Routing Screen

Routing object:

```text
Policy Name
Strategy
Primary Targets
Fallback Targets
Health Requirement
Retry
Timeout
Circuit Breaker
```

Supported strategies:

```text
Priority
Weighted
Round Robin
Lowest Latency
Lowest Error Rate
Manual
9Router Combo
```

The UI MUST explain why a route is selected when an operator opens a routing decision.

---

## 13. Proxies Screen

Display:

```text
Profile
Type
Host
Port
Health
Latency
Assigned Targets
Last Check
```

Credentials are never rendered.

Safe display:

```text
SOCKS5-01
socks5
proxy.example:1080
credentials: configured
status: ● healthy
```

Actions:

```text
A Add
E Edit
T Test
H Health
D Disable
```

---

## 14. Usage Screen

Views:

```text
Today
7 Days
30 Days
Custom
```

Dimensions:

```text
Requests
Input Tokens
Output Tokens
Total Tokens
Estimated Cost
Model
Provider
API Key
```

Estimated values MUST be explicitly marked as estimated.

---

## 15. Audit Screen

Columns:

```text
TIME
ACTOR
ACTION
TARGET
RESULT
```

Detail:

```text
Actor
Role
Action
Target
Timestamp
Correlation ID
Result
Change summary
```

Secrets MUST never be rendered.

---

## 16. Settings Screen

Sections:

```text
Gateway
9Router
Database
Redis
Rate Limits
Logging
Metrics
Security
TUI
```

Settings requiring restart must show:

```text
RESTART REQUIRED
```

---

## 17. Modal Design

All modals use a consistent pattern:

```text
╭─ EDIT PROVIDER ───────────────────────────────╮
│ Name      OpenCode                            │
│ Base URL  _________________________________   │
│ Priority  10                                  │
│ Enabled   ● Yes                               │
│                                              │
│ [Cancel]                         [Save]       │
╰──────────────────────────────────────────────╯
```

Destructive modal:

```text
╭─ CONFIRM DISABLE ─────────────────────────────╮
│ Disable provider "Anthropic"?                │
│                                              │
│ 3 routes will lose a preferred target.       │
│                                              │
│ [Cancel]                        [Disable]     │
╰──────────────────────────────────────────────╯
```

The destructive action MUST use warning/error semantics without excessive visual emphasis.

---

## 18. Empty / Loading / Error States

### Empty

```text
No providers configured.

[A] Add provider
```

### Loading

```text
Loading providers...
```

Animation SHOULD be subtle.

### Error

```text
Unable to load providers.

Reason: 9Router management API unavailable

[R] Retry   [B] Back
```

Errors must provide a next action.

---

## 19. Accessibility

Requirements:

- no information may depend only on color;
- keyboard-only operation;
- consistent focus state;
- visible selection;
- readable minimum contrast;
- text alternatives for symbols;
- terminal capability detection;
- graceful fallback when colors are unavailable.

---

## 20. Responsive Behavior

Priority under terminal shrink:

1. status
2. selected object
3. primary action
4. health
5. essential metrics
6. secondary metadata
7. verbose descriptions

Long model/provider names should be ellipsized with detail-on-open.

---

## 21. Interaction Standards

Use consistent keys:

```text
Enter   Open
Esc     Back
E       Edit
A       Add
D       Disable/Delete
S       Switch
T       Test
R       Refresh/Retry depending on context
/       Search
?       Help
Q       Quit
```

The same key MUST NOT mean unrelated destructive operations in neighboring screens without confirmation.

---

## 22. Animation

Animations are optional and should be subtle.

Allowed:

- progress pulse;
- loading indicator;
- transient status transition.

Not allowed:

- flashing content;
- persistent blinking;
- large moving decorations;
- attention-grabbing effects.

---

## 23. Implementation Rules

The UI must implement a reusable design system:

```text
StatusBadge
MetricCard
DataTable
SearchBar
Modal
FormField
Toast
ConfirmDialog
HelpOverlay
LogViewer
Sparkline
ProgressBar
```

All components must use centralized theme tokens.

No screen may hardcode arbitrary colors.

---

## 24. UX Acceptance Criteria

The UI baseline is accepted when:

- all P0 management actions are reachable by keyboard;
- no 3D or decorative iconography is used;
- color usage is subdued and consistent;
- 80x24 remains usable;
- every destructive action confirms;
- secrets never appear in normal views;
- all management mutations produce audit events;
- TUI uses only the ProxyGateway API;
- error states include recovery guidance.

---

## 25. Relationship to Engineering Documents

The implementation MUST comply with:

- `01-PRD.md` for functional requirements;
- `03-ARCHITECTURE.md` for service and data boundaries;
- `04-TECH-STACK.md` for technical choices;
- `05-REFACTORY-PLAN.md` for module boundaries;
- `07-IMPLEMENTATION-DOCUMENTATION.md` for traceability.

Any deviation must be recorded in the implementation documentation and, where architectural, in an ADR.
