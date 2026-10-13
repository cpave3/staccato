# Staccato coworker demo

`storyboard.json` is the recording studio's source of truth. It contains the
agreed nine-scene plan, talking points and pictures, but no scripted narration.
Record each scene in Legato's studio; align visual actions to the takes afterward.

## Evidence and limitations

- The CLI was built from this checkout. Command output in `assets/captures.json`
  came from disposable, isolated local repositories with no project hooks.
  The `repo` and `worktree` paths identify those temporary repositories; they
  are not required to open the storyboard.
- Stack creation, modification, intentional conflict recovery, sync after a
  bottom-branch merge, per-branch tests and a verified split were actually run.
- Sync used a local bare `origin`. A second clone fast-forwarded the API into
  trunk and deleted its remote branch to simulate the merged PR. No GitHub PR
  was created; the PR commands are explicitly labelled as not executed.
- `assets/split-plan.json` is the plan actually applied. Its head SHA belongs to
  the disposable example, not this repository. It is evidence, not a plan to
  apply to your own work.
- Captures combine stdout then stderr, not their chronological interleaving.
  Visuals wrap long lines and label excerpts. The exec scene shows the stdout
  report, omitting live test stderr. Diagram labels summarize the observed state.
- `st sync --down` fetches and updates the stack without pushing; it is not a
  local-only check. Automatic backups are described for restack operations,
  not as a blanket guarantee for every destructive Git action.

Validation: `libretto validate demos/staccato/storyboard.json`. A separate,
silent picture-only preview was rendered and inspected to check layout;
that preview is not the narrated video. The storyboard still waits for takes.
