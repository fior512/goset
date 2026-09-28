The Goset tool is an open project written in Go. Contributions are welcomed. Contributors must agree to the project's license.

---

Wanted:
- Bug fixes
- New features that serve a majority of users
- Readability changes
- Documentation
- Security
- Portability

Not wanted:
- Multi-module refactors
- New formatting style
- New dependencies

---

Issues:
- A report starts from the issue form that matches it: a bug report or a feature request.
- One concern per issue. A request that needs more than one arrives as a chain of issues.

---

Code style: structure
- A function hides one level of abstraction (SLA). It does not leak that level to its caller. A function is the `main()` of its level of detail.
- A pipeline reads as a sequence in the caller: `setup(); run(); report();`. Depth lives in the callees, not in nested calls (stepdown rule).
- A function is self-contained. It builds the structures its own job needs. Everything it needs is inside it. No dangling logic in the caller above the call.
- A caller must not do a callee's setup, translation, or init. That work stays inside the callee.
- A helper lives in the file of the code it serves, not in the file of its caller (locality of behavior).

Code style: naming
- A function name states the purpose of its logic, not its mechanics (intention-revealing name).
- A structure object keeps the same name at every callsite (`name := Structure{}` reusing `name`).
- One-letter object names are forbidden (other than loop idx).

Code style: comments
- Prose comments are forbidden. A comment states only a technical or logical cue about the code.
- A one-word `/* label */` may segment two blocks of logic inside a single function.

Code style: errors
- Code without proper error handling is forbidden.
- Do not handle a state that cannot occur.

Code style: misc
- Non-ASCII characters are forbidden, in code, documentation, commit messages, and issue or PR text.
- Messy code (wrappers of wrappers, logic bleeding outside modules/layers, ..) is forbidden.

---

Text:
- Markdown is never hard-wrapped. A line runs to the end of its paragraph, the renderer wraps it.
- Issue and PR bodies are terse and objective on the task.

---

Git:
- Branch roots follow the classic convention: `feat/`, `fix/`, `doc/`, ..
- Commit messages must include enough information to use `git bisect`.
- A PR is about a single aspect. Use stacked PRs to break down every change independently.

A PR description must state:
- the current state
- why it changes
- what the implementation is
- the output of `go build ./...` and `go test ./...`, or a real run's telemetry, showing the change works

Each section carries one job:
- `Current State` states the defect, not how the defect appeared
- `Why it changes` states the consequence, in one or two sentences
- `What the implementation is` is one bullet per change, and no bullet restates what the patch already shows
- `Output` carries verbatim command output, and any limit as a fact

The first line is an issue reference when the change has one, and is absent when it does not. No line is ever written for the missing case.

A commit body wraps at 72 columns. A markdown document does not wrap. They are different artifacts under different rules.

If the author cannot explain any line of their own PR on request, the PR is discarded.

If the benefit of your PR remains unclear, or too specialized for a single use case, we may discard it or leave it waiting.

---

AI policy
AI assistance is allowed if the author remains accountable for every line, per the accountability rule above.
Low-quality code, even if implementing a major feature or fixing a bug, will always be discarded.
[see AI Policy](AGENTS.md)
