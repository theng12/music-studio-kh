# Execution ledger — 2026-10-12-foundation.md

User explicitly requested verification and starting the build; implement the runnable foundation inline.
Repository: theng12/tuneharbor, private, write access verified. Approved docs published to main.
Branch: feat/tuneharbor-foundation. Existing checkout is dedicated to this new project; use a feature branch in place rather than request additional worktree approval.
Pre-flight: profile → library → server → UI interfaces match the plan.
Ruling: use an embedded lightweight browser player for the first executable milestone; React/Tauri remains planned for the desktop release. Cost: porting UI components later.
Ruling: first Subsonic implementation supports password authentication, not token/salt; encrypted compatibility credentials require desktop OS keychain integration. Cost: some mobile clients cannot connect until the next milestone; document explicitly.
