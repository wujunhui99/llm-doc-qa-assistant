# Agent Self-Testing Spec

## Goal
Agent must be able to test its own code changes before completion, including reproducible UI checks.

## Required Capabilities
1. Worktree-isolated app boot:
- Each change can run in its own `git worktree`.
- Agent can start one app instance per worktree without polluting other changes.

2. CDP-enabled runtime:
- Agent runtime must connect to Chrome DevTools Protocol (CDP).
- CDP session is used for deterministic browser inspection and interaction.

3. Browser skill set:
- DOM snapshot capture
- Screenshot capture
- Navigation/action execution

## Required Agent Behaviors
- Reproduce reported UI bug steps in browser.
- Capture before-fix evidence (DOM snapshot + screenshot).
- Apply fix and re-run same steps.
- Capture after-fix evidence and determine pass/fail.

## Evidence Artifacts
For each self-test run, agent should produce:
- worktree identifier
- navigation/action log
- DOM snapshot references
- screenshots (before/after)
- final validation verdict

## Acceptance Criteria
- Agent can run UI validation in isolated worktree environments.
- Agent can use CDP to inspect/act on UI.
- Agent can reproduce at least one known UI issue and verify fix with artifacts.
