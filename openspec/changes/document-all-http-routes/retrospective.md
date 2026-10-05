# Retrospective — document-all-http-routes

## What worked

Inventory by method/path exposed six missing operations despite existing tests checking representative paths. Served-document regressions reproduced the omissions before implementation and now guard complete explicit-route coverage. Reusing envelope composition preserved existing handlers while correcting generated client schemas.

One implementation batch kept annotation conventions consistent across routes. An isolated worktree and a baseline snapshot separated task changes from substantial existing local edits. Independent task and broad final reviews approved the result; the full default Go gates passed.

Delivery checked destination bytes against the original snapshot, applied only the reviewed delta and verified the resulting bytes. The original workspace then passed the focused served-Swagger checks and reproduced the generated artifacts exactly. The worktree remains available for later integration decisions.

## Adjustments and lessons

The pinned generator initially exposed inaccurate quota field interpretation through custom decoder types; documentation-only type metadata corrected the generated schemas without changing decoding. Swagger 2.0's binary-body and cookie limitations require explicit descriptions, while webhook/HTML/no-body responses must be treated according to their real formats.

Comparing production function tokens avoids false differences caused by comment positions in formatted AST output. Generation freshness compares output bytes directly because a dirty baseline makes `git diff HEAD -- docs` unsuitable as this task's local freshness proof; CI retains its clean-checkout diff guard.

The README regeneration example uses portable `go run`; this machine's PATH override belongs to local verification commands.

## Remaining boundaries

External integrations and live-service rollout are outside this documentation change. Baseline functional issues identified during earlier analysis remain separate work. New routes using registration helpers or another file must extend the coverage inventory accordingly.
