package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// selfExempt is the checker itself. The prose rule governs product code. A
// linter states its rules in godoc, and a rule about comments cannot be read
// without them.
const selfExempt = ".github/lint/"

// proseComments flags a comment that states a sentence about the code.
// CONTRIBUTING allows a comment to carry only a technical or logical cue, so a
// hit is one of two shapes: a comment that closes a sentence, or a comment
// that opens with the identifier it documents and then narrates it. A cue that
// locates code, a path, a URL, or a flag is never a hit. A comment that sits
// after code on its line is a cue by position, and is skipped.
func proseComments(file *ast.File, fset *token.FileSet, src []byte, found *violationList) {
	if strings.Contains(fset.Position(file.Pos()).Filename, selfExempt) {
		return
	}
	source := strings.Split(string(src), "\n")
	documented := documentedNames(file)
	for _, group := range file.Comments {
		start := fset.Position(group.Pos())
		column := start.Column
		var reasons []string
		for offset, line := range strings.Split(group.Text(), "\n") {
			text := commentText(line)
			if text == "" {
				continue
			}
			if isTrailing(source, start.Line+offset, offset, column) {
				continue
			}
			if hit, reason := proseLine(text, documented[group]); hit {
				reasons = append(reasons, reason+": "+text)
			}
		}
		if len(reasons) > 0 {
			found.add("prose-comment", start, strings.Join(reasons, " | "))
		}
	}
}

// isTrailing reports a comment line that sits after code on its own line. The
// first line of a group starts at the comment's column, so only the text
// before it counts. A later line is inside a block comment and owns its line.
func isTrailing(source []string, line, offset, column int) bool {
	if offset > 0 {
		return false
	}
	if line < 1 || line > len(source) || column <= 1 {
		return false
	}
	return strings.TrimSpace(source[line-1][:column-1]) != ""
}

// commentText strips the comment markers a line is wrapped in.
func commentText(line string) string {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"///", "//", "*/", "*"} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return line
}

// documentedNames maps each doc comment group to the identifiers it
// documents. A group can serve a declaration and its spec at once.
func documentedNames(file *ast.File) map[*ast.CommentGroup][]string {
	documented := map[*ast.CommentGroup][]string{}
	record := func(doc *ast.CommentGroup, names ...string) {
		if doc == nil {
			return
		}
		documented[doc] = append(documented[doc], names...)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncDecl:
			record(typed.Doc, typed.Name.Name)
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch inner := spec.(type) {
				case *ast.TypeSpec:
					record(inner.Doc, inner.Name.Name)
					record(typed.Doc, inner.Name.Name)
				case *ast.ValueSpec:
					record(inner.Doc, fieldNames(inner.Names)...)
					record(typed.Doc, fieldNames(inner.Names)...)
				case *ast.ImportSpec:
					if inner.Name != nil {
						record(inner.Doc, inner.Name.Name)
					}
				}
			}
		case *ast.Field:
			record(typed.Doc, fieldNames(typed.Names)...)
		}
		return true
	})
	return documented
}

// fieldNames lists the identifiers a declaration binds.
func fieldNames(idents []*ast.Ident) []string {
	names := make([]string, 0, len(idents))
	for _, ident := range idents {
		names = append(names, ident.Name)
	}
	return names
}

// proseLine reports whether one comment line is a sentence, and why. names are
// the identifiers the comment documents, empty for a free comment.
func proseLine(line string, names []string) (bool, string) {
	if isDirective(line) || isPin(line) {
		return false, ""
	}
	if narratesName(line, names) {
		return true, "narrated identifier"
	}
	if hasTerminalMark(line) {
		return true, "terminal mark"
	}
	return false, ""
}

// isDirective covers tool pragmas: go:embed, go:generate, nolint, build.
func isDirective(line string) bool {
	return strings.HasPrefix(line, "go:") ||
		strings.HasPrefix(line, "nolint") ||
		strings.HasPrefix(line, "+build") ||
		strings.HasPrefix(line, "line ") ||
		strings.HasPrefix(line, "export ")
}

// isPin covers a cue that locates code instead of describing it: a URL, a
// filesystem path, a source reference, a flag, or a numeric literal.
func isPin(line string) bool {
	for _, field := range strings.Fields(line) {
		if isPinField(field) {
			return true
		}
	}
	return false
}

// isPinField classifies one whitespace-separated token of a comment.
func isPinField(field string) bool {
	trimmed := strings.Trim(field, "`()[];,:\x22'")
	switch {
	case trimmed == "":
		return false
	case strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"):
		return true
	case strings.HasPrefix(trimmed, "/"):
		return true
	case strings.HasPrefix(trimmed, "-"):
		return true
	case strings.Contains(trimmed, ".go:"), strings.HasSuffix(trimmed, ".go"):
		return true
	case strings.Contains(trimmed, ".md:"), strings.HasSuffix(trimmed, ".md"):
		return true
	case strings.HasPrefix(trimmed, "0o"), strings.HasPrefix(trimmed, "0x"):
		return true
	}
	return false
}

// narratesName reports a doc comment that opens with the identifier it
// documents and then keeps going. The name is the whole cue; the rest is
// narration, and CONTRIBUTING forbids prose.
func narratesName(line string, names []string) bool {
	words := strings.Fields(line)
	if len(words) < 2 {
		return false
	}
	head := strings.Trim(words[0], "`")
	for _, name := range names {
		if head == name {
			return true
		}
	}
	return false
}

// hasTerminalMark reports a mark that closes a sentence. A mark inside a
// decimal or a dotted identifier does not close one, so the mark must be the
// last character of the line, past a word.
func hasTerminalMark(line string) bool {
	last := line[len(line)-1]
	if last != '.' && last != '!' && last != '?' {
		return false
	}
	before := strings.TrimRight(line[:len(line)-1], " ")
	if before == "" {
		return false
	}
	tail := before[strings.LastIndex(before, " ")+1:]
	return !isPinField(tail) && !strings.HasSuffix(tail, "v2") && !strings.HasSuffix(tail, "v1")
}
