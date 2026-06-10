# Feature Specification: IaaS Provider HTTP Timeout

**Feature Branch**: `004-iaas-http-timeout`

**Created**: 2026-06-02

**Status**: Draft

**Input**: User description: "Add a configuration in Spiderpool's IaaS provider integration for HTTP interaction timeout, configurable via Helm values. When Spiderpool calls the provider, set the timeout which must not exceed 2 minutes because kubelet's CNI ADD default timeout is 2 minutes. However, this 2-minute default can be changed, and CNI DEL has different timeout behavior, so it should not be hardcoded. Currently Spiderpool reuses the parent context when calling the provider; check if the parent context's timeout can be retrieved, then compare it with the configured HTTP timeout - the HTTP timeout must be smaller than the parent context's timeout."

## Clarifications

### Session 2026-06-02

- Q: Can Spiderpool obtain the provider call's parent timeout from context? -> A: Only a context deadline can be obtained when present; current Spiderpool CNI-to-agent calls use a derived internal deadline, so the plan must verify deadline propagation instead of assuming kubelet's original timeout is visible.
- Q: What parent budget should the provider HTTP timeout be compared against in the current Spiderpool CNI flow? -> A: Compare against the current CNI plugin-to-agent timeout of 100 seconds; ADD and DEL both use this value today, and this timeout is not currently configurable. If that timeout becomes configurable later, startup validation must compare the provider HTTP timeout with the configured CNI client timeout.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Provider Timeout (Priority: P1)

As a cluster administrator using Spiderpool with an IaaS provider, I want to
configure the provider HTTP interaction timeout through the Spiderpool Helm
values so that provider calls cannot consume the whole CNI operation budget.

**Why this priority**: This is the primary safety control. Without it, a slow
provider can make Spiderpool wait until the parent CNI operation expires.

**Independent Test**: Install or render Spiderpool with a valid timeout value and
verify the value is accepted, documented, and available to the components that
call the provider.

**Acceptance Scenarios**:

1. **Given** IaaS provider integration is enabled, **When** the administrator
   configures a valid provider timeout in Helm values, **Then** Spiderpool uses
   that value for provider interactions.
2. **Given** IaaS provider integration is disabled, **When** the administrator
   leaves the timeout unset, **Then** Spiderpool continues to install and operate
   without requiring a provider timeout.

---

### User Story 2 - Prevent Unsafe Timeout Values (Priority: P1)

As a cluster administrator, I want Spiderpool to reject or report provider
timeout values that are not safely below the caller's available time budget so
that CNI ADD, CNI DEL, and cleanup flows fail predictably instead of expiring at
the outer deadline.

**Why this priority**: The configured timeout must not be equal to or greater
than Spiderpool's CNI plugin-to-agent operation timeout. In the current code,
CNI ADD and CNI DEL both use a 100-second timeout at that boundary.

**Independent Test**: Run provider allocation and release flows with the current
CNI plugin-to-agent timeout and verify Spiderpool only starts provider calls when
the configured provider timeout is shorter than that budget.

**Acceptance Scenarios**:

1. **Given** a parent CNI ADD operation uses the current 100-second
   plugin-to-agent timeout, **When** the configured provider timeout is shorter
   than that budget,
   **Then** Spiderpool calls the provider using the configured timeout.
2. **Given** a parent CNI ADD or CNI DEL operation uses the current 100-second
   plugin-to-agent timeout, **When** the configured provider timeout is equal to
   or greater than that budget, **Then** Spiderpool rejects the configuration or
   does not start an unsafe provider call.
3. **Given** the CNI plugin-to-agent timeout becomes configurable, **When**
   Spiderpool validates the provider timeout, **Then** Spiderpool compares it
   with the configured CNI client timeout rather than a literal 100-second value.

---

### User Story 3 - Diagnose Timeout Decisions (Priority: P2)

As an operator troubleshooting IaaS provider calls, I want Spiderpool to report
clear timeout-related errors and observations so that I can tell whether a call
timed out, was rejected because the configured timeout was unsafe, or proceeded
with a valid budget.

**Why this priority**: Timeout failures affect Pod startup and cleanup. Clear
diagnostics reduce recovery time and avoid misconfiguring provider integration.

**Independent Test**: Trigger provider call timeout, unsafe timeout, and valid
timeout cases and verify the operator-facing messages distinguish the outcomes.

**Acceptance Scenarios**:

1. **Given** a provider call exceeds the configured provider timeout, **When**
   Spiderpool reports the failure, **Then** the message identifies that the
   provider interaction timed out.
2. **Given** Spiderpool rejects a provider call because its configured timeout is
   not below the parent budget, **When** the failure is reported, **Then** the
   message includes the configured provider timeout and the available parent
   budget.

### Edge Cases

- Parent operation has no discoverable deadline: Spiderpool uses the configured
  provider timeout and still applies static configuration validation.
- Parent context carries an internal Spiderpool deadline rather than the
  runtime's original CNI timeout: Spiderpool compares against the available
  context deadline and the plan verifies whether runtime timeout propagation
  must be added.
- Current CNI ADD and CNI DEL plugin-to-agent calls both use the same
  100-second timeout. The provider HTTP timeout must be strictly smaller than
  that value until a separate Spiderpool CNI client timeout configuration exists.
- A future Spiderpool CNI client timeout setting is added or discovered:
  component startup validation must compare the provider HTTP timeout with that
  configured CNI client timeout.
- IPAM release derives a shorter operation context from Pod deletion grace
  period after the CNI DEL request reaches the agent: provider release calls
  must still honor the current operation context deadline at call time.
- Parent operation has already expired or has no remaining time: Spiderpool does
  not start the provider call and reports that the parent budget is exhausted.
- Configured provider timeout is zero, negative, unparsable, or missing while
  IaaS provider integration is enabled: Spiderpool reports an invalid
  configuration or uses the documented default.
- Configured provider timeout equals the parent remaining budget exactly:
  Spiderpool treats it as unsafe because the provider timeout must be smaller.
- CNI ADD and CNI DEL expose different parent budgets in a future change:
  Spiderpool evaluates each call against the budget of the current operation.
- Runtime scheduling delay leaves less remaining parent time than was available
  when the operation started: Spiderpool compares against the remaining time at
  the point the provider call is made.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide an IaaS provider HTTP timeout setting that can
  be configured through Spiderpool Helm values.
- **FR-002**: System MUST document the timeout setting, accepted duration format,
  default value, and invalid-value behavior for operators.
- **FR-003**: System MUST apply the configured timeout to every Spiderpool
  interaction with the IaaS provider, including allocation and release flows.
- **FR-004**: System MUST preserve existing behavior when IaaS provider
  integration is disabled.
- **FR-005**: System MUST validate that configured timeout values are positive
  and do not exceed the documented static safety limit.
- **FR-006**: System MUST compare the configured provider timeout with the
  current operation's remaining context deadline whenever that deadline is
  available.
- **FR-007**: System MUST require the configured provider timeout to be strictly
  smaller than the current 100-second CNI plugin-to-agent timeout before
  accepting the configuration for CNI-triggered provider calls.
- **FR-008**: System MUST validate the provider HTTP timeout during component
  startup when IaaS provider integration is enabled.
- **FR-009**: System MUST use a configured Spiderpool CNI client timeout for the
  comparison if that timeout becomes configurable; otherwise the current
  100-second ADD/DEL timeout is the comparison budget.
- **FR-010**: System MUST confirm CNI ADD and CNI DEL timeout behavior in tests;
  today both operations use the same 100-second plugin-to-agent timeout.
- **FR-011**: System MUST handle controller cleanup and IPAM release flows
  according to their current operation context, because these flows may have
  budgets that differ from the CNI plugin-to-agent timeout.
- **FR-012**: System MUST return or log a clear operator-facing error when the
  configured timeout is invalid, unsafe for the current parent budget, or reached
  during a provider call.
- **FR-013**: System MUST preserve existing Spiderpool API, CRD, Helm,
  annotation, and webhook behavior unless an explicit compatibility exception is
  documented.
- **FR-014**: System MUST expose user/operator-facing names, defaults,
  validation errors, status fields, and examples consistently with existing
  Spiderpool conventions.

### Key Entities

- **IaaS Provider Timeout Setting**: Operator-configured duration controlling how
  long Spiderpool may wait for one provider HTTP interaction.
- **Parent Operation Budget**: Remaining time available from the current
  operation context, such as CNI ADD, CNI DEL, or controller cleanup, when
  Spiderpool is about to contact the provider.
- **CNI Plugin-to-Agent Timeout**: Spiderpool's timeout for CNI plugin requests
  sent to the Spiderpool Agent. The current ADD and DEL value is 100 seconds.
- **Provider Call Outcome**: Result of a provider interaction, including success,
  provider timeout, parent-budget rejection, invalid configuration, or provider
  error.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of provider allocation and release calls use the configured
  provider timeout when IaaS provider integration is enabled.
- **SC-002**: 100% of provider calls with a discoverable parent budget are
  rejected before start when the configured provider timeout is equal to or
  greater than the remaining parent budget.
- **SC-003**: CNI ADD and CNI DEL validation tests confirm both operations use
  the same 100-second plugin-to-agent timeout today and reject provider HTTP
  timeout values that are equal to or greater than that value.
- **SC-004**: Operator-facing documentation includes the Helm value name, default,
  valid examples, invalid examples, and the requirement that the provider timeout
  be smaller than the parent operation budget.
- **SC-005**: Invalid or unsafe timeout configurations produce distinct messages
  that identify the configured timeout and the relevant limit or parent budget.
- **SC-006**: When IaaS provider integration is disabled, the timeout setting does
  not change successful Pod allocation or release outcomes.

## Assumptions

- The feature extends the existing IaaS provider integration rather than adding a
  second provider integration mode.
- The default provider timeout is defined by the project during planning and is
  below the documented static safety limit.
- The static safety limit remains two minutes unless the project exposes a
  separate supported configuration for changing that limit.
- Current Spiderpool CNI ADD and CNI DEL plugin-to-agent calls both use a
  100-second timeout and this value is not currently exposed through Helm values
  or runtime configuration.
- Parent operation budget is determined from the current context deadline when
  that deadline is exposed to Spiderpool.
- Context can reveal a deadline, not the original configured timeout value. The
  plan must verify whether the current deadline represents kubelet's runtime
  budget or a Spiderpool-derived internal budget.
- If no context deadline is exposed, Spiderpool cannot compare against a dynamic
  parent budget and relies on the configured provider timeout plus static
  validation.
