Read CONTRIBUTING.md before any change. Its rules apply to AI-assisted contributions the same as human ones.

Build: `go build ./...`
Test: `go test ./...`
Run both before reporting a task done. Report any skipped check.

Drafting a PR description is allowed, the human author owns and edits it. Do not draft replies to other contributors. If they cannot explain their own commit, the PR is discarded.

Writing:
- The first sentence of a section is the whole section. No buildup, no preamble, no "currently".
- One claim per sentence, 25 words maximum. Two claims joined by "and" are two sentences, or two bullets.
- Delete any sentence whose only job is to justify the next sentence. Keep the consequence, drop the justification.
- Do not restate a diff. If the patch already says it, the text does not.
- State a limit as a fact. "needs root", not "is not exercised locally on a non root box".
- One word per concept across the repository. No synonym chain.
- Numbers, not adjectives. "3 files, 35 insertions", not "a small focused change".
- Present tense, active voice, short subjects. "reads no", not "was reporting as no".
- One screen, about 200 words, excluding verbatim command output.
- Delete the closing paragraph. A caveat that needs a paragraph needs a line.

What a change must contain is in CONTRIBUTING.md. These rules are how to phrase it.

AI Disclosure is not mandatory, and does not replace the author's accountability for every line.
