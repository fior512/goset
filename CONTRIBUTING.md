The Goset tool is an open project written in Go. Contributions are welcomed. Contributors must agree to the project's license.

In particular:
- Bug fixes
- New features that serve a majority of users
- Readability changes (segmenting logical modules and depths of details)
- Documentation
- Security
- Portability

On the other side, we discourage:
- Multi-module refactors
- New formatting style
- New dependencies
---

Forbidden code-style:
- Messy code (wrappers of wrappers, logic bleeding outside modules/layers, ..)
- A function hides one level of abstraction. It does not leak that level to its caller.
- A function is self-contained. It builds the structures its own job needs.
- A caller must not do a callee's setup, translation, or init. That work stays inside the callee.
- Non-ASCII characters
- Prose comments. (A comment must only be written for technical or logical cues about the code)
- One-letter object names (other than loop idx)
- Code without proper error handling
- Error handling for a state that cannot occur
- A structure object must keep the same name at every callsite (`name := Structure{}` reusing `name`)

---
Branch naming
We follow the classic convention of branch roots `feat/`, `fix/`, `doc/`, ..
Commit messages must include enough information to use `git bisect`.
A PR must be about a single aspect, use the stacked-PRs GitHub feature to break down every change independently.

A PR description must state:
- the current state
- why it changes
- what the implementation is
- the output of `go build ./...` and `go test ./...`, or a real run's telemetry, showing the change works

If the author cannot explain any line of their own PR on request, the PR is discarded.

---
If the benefit of your PR remains unclear, or too specialized for a single use case, we may discard it or leave it waiting.

---
AI policy
AI assistance is allowed if the author remains accountable for every line, per the accountability rule above.
Low-quality code, even if implementing a major feature or fixing a bug will always be discarded.
[see AI Policy](AGENTS.md)
