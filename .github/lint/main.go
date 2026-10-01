// goset-lint holds the checks that gate a change: capitalized locals,
// one-letter locals, prose comments. Each maps to a CONTRIBUTING code style
// rule. Run it over the whole tree, or against one revision to gate only the
// lines a change touches.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// check is one style rule, named after the CONTRIBUTING rule it enforces.
type check struct {
	name string
	run  func(*ast.File, *token.FileSet, []byte, *violationList)
}

// violation is one rule hit, located by file and line for a readable CI log.
type violation struct {
	rule string
	pos  token.Position
	text string
}

// violationList accumulates hits, one per rule per position.
type violationList struct {
	seen  map[string]bool
	items []violation
}

func newViolationList() *violationList {
	return &violationList{seen: map[string]bool{}}
}

func (l *violationList) add(rule string, pos token.Position, text string) {
	key := fmt.Sprintf("%s|%d|%s", rule, pos.Line, text)
	if l.seen[key] {
		return
	}
	l.seen[key] = true
	l.items = append(l.items, violation{rule: rule, pos: pos, text: text})
}

func main() {
	root := flag.String("root", ".", "root directory to walk")
	revision := flag.String("diff", "", "git revision: report only the lines it does not already own")
	flag.Parse()

	files, err := goFiles(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	added, err := addedLines(*revision, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	fset := token.NewFileSet()
	checks := []check{
		{"capitalized-local", capitalizedLocals},
		{"one-letter-local", oneLetterLocals},
		{"prose-comment", proseComments},
	}

	found := newViolationList()
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(2)
		}
		file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(2)
		}
		for _, rule := range checks {
			rule.run(file, fset, src, found)
		}
	}

	sortViolations(found)

	reported := 0
	for _, item := range found.items {
		if !owns(added, item.pos) {
			continue
		}
		fmt.Printf("%s:%d: %s: %s\n", relative(item.pos.Filename, *root), item.pos.Line, item.rule, item.text)
		reported++
	}

	counts := map[string]int{}
	for _, item := range found.items {
		counts[item.rule]++
	}
	fmt.Fprintf(os.Stderr, "goset-lint: %d tracked, %d introduced\n", len(found.items), reported)
	for _, rule := range checks {
		fmt.Fprintf(os.Stderr, "  %-20s %d\n", rule.name, counts[rule.name])
	}
	if reported > 0 {
		os.Exit(1)
	}
}

// sortViolations puts hits in file then line order, so a log reads top down.
func sortViolations(found *violationList) {
	sort.SliceStable(found.items, func(left, right int) bool {
		if found.items[left].pos.Filename != found.items[right].pos.Filename {
			return found.items[left].pos.Filename < found.items[right].pos.Filename
		}
		return found.items[left].pos.Line < found.items[right].pos.Line
	})
}

// owns reports whether a position is inside the lines a revision introduced.
// An empty revision owns every line, which is a whole-tree run.
func owns(added map[string]map[int]bool, pos token.Position) bool {
	if added == nil {
		return true
	}
	lines, ok := added[pos.Filename]
	if !ok {
		return false
	}
	return lines[pos.Line]
}

// addedLines reads the added line numbers of a revision against the worktree
// head. A nil map means no revision, so nothing is filtered. A file git does
// not track yet has no diff, so every line of it counts as added.
func addedLines(revision, root string) (map[string]map[int]bool, error) {
	if revision == "" {
		return nil, nil
	}
	out, err := exec.Command("git", "-C", root, "diff", "--unified=0", revision, "--", "*.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", revision, err)
	}
	added := map[string]map[int]bool{}
	file, first, count := "", 0, 0
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			file = strings.TrimPrefix(line, "+++ b/")
			if added[file] == nil {
				added[file] = map[int]bool{}
			}
		case strings.HasPrefix(line, "@@"):
			first, count = parseHunk(line)
		case strings.HasPrefix(line, "+") && file != "":
			for offset := 0; offset < count; offset++ {
				added[file][first+offset] = true
			}
		}
	}
	return addUntracked(added, root)
}

// addUntracked marks every line of a .go file git does not track as added, so
// a new file is gated like any other change.
func addUntracked(added map[string]map[int]bool, root string) (map[string]map[int]bool, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "--others", "--exclude-standard", "--", "*.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	for _, name := range strings.Fields(string(out)) {
		lines, err := countLines(root, name)
		if err != nil {
			return nil, err
		}
		owned := map[int]bool{}
		for line := 1; line <= lines; line++ {
			owned[line] = true
		}
		added[name] = owned
	}
	return added, nil
}

// countLines counts the lines a file holds, so every one of them is owned.
func countLines(root, name string) (int, error) {
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return 0, err
	}
	return strings.Count(string(data), "\n") + 1, nil
}

// parseHunk reads the new-file start and length out of a diff hunk header,
// whose range carries a leading plus: "+start,length".
func parseHunk(header string) (int, int) {
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return 0, 0
	}
	span := strings.TrimPrefix(fields[2], "+")
	first, size, hasSize := strings.Cut(span, ",")
	start, err := atoi(first)
	if err != nil {
		return 0, 0
	}
	count := 1
	if hasSize {
		value, err := atoi(size)
		if err != nil {
			return 0, 0
		}
		count = value
	}
	return start, count
}

// atoi parses a non-negative base-ten integer.
func atoi(text string) (int, error) {
	value := 0
	for _, digit := range text {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("not a number: %s", text)
		}
		value = value*10 + int(digit-'0')
	}
	return value, nil
}

// relative shortens a path for the log, so a hit reads as repo-relative.
func relative(path, root string) string {
	prefix := strings.TrimSuffix(root, "/") + "/"
	return strings.TrimPrefix(path, prefix)
}

// goFiles lists every .go file under root, skipping VCS and output dirs.
func goFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// capitalizedLocals flags a function-local name that starts uppercase. The
// capital is the export marker, and a local has no package to export to.
func capitalizedLocals(file *ast.File, fset *token.FileSet, _ []byte, found *violationList) {
	ast.Inspect(file, func(node ast.Node) bool {
		body := funcBody(node)
		if body == nil {
			return true
		}
		for _, local := range localNames(body) {
			if startsUpper(local.name) {
				found.add("capitalized-local", fset.Position(local.pos), local.name)
			}
		}
		return true
	})
}

// oneLetterLocals flags a one-letter local. CONTRIBUTING excepts a loop index,
// which is the one place a short name buys nothing.
func oneLetterLocals(file *ast.File, fset *token.FileSet, _ []byte, found *violationList) {
	ast.Inspect(file, func(node ast.Node) bool {
		body := funcBody(node)
		if body == nil {
			return true
		}
		for _, local := range localNames(body) {
			if local.name == "i" && local.role == roleIndex {
				continue
			}
			if local.name == "t" && local.testHandle {
				continue
			}
			if isOneLetter(local.name) {
				found.add("one-letter-local", fset.Position(local.pos), local.name)
			}
		}
		return true
	})
}

// localRole tells a loop index from everything else, so the index exception
// applies to the For and Range statements that own the index.
type localRole int

const (
	roleOther localRole = iota
	roleIndex
)

// local is one declared name with the role of its declaration site.
type local struct {
	name       string
	pos        token.Pos
	role       localRole
	testHandle bool
}

// localNames collects the names a function body binds: assign definitions,
// var and const declarations, range keys and values, type-switch bindings,
// and the parameters of the function and of every literal nested in it. A name
// first seen as a loop index keeps that role when the walk reaches it again
// through the plain assignment case.
func localNames(body *ast.BlockStmt) []local {
	roles := map[token.Pos]localRole{}
	handles := map[token.Pos]bool{}
	var order []token.Pos
	names := map[token.Pos]string{}
	bind := func(expr ast.Expr, role localRole) {
		ident, ok := expr.(*ast.Ident)
		if !ok || ident.Name == "_" {
			return
		}
		if _, known := names[ident.Pos()]; !known {
			order = append(order, ident.Pos())
			names[ident.Pos()] = ident.Name
			roles[ident.Pos()] = role
		}
		if role == roleIndex {
			roles[ident.Pos()] = roleIndex
		}
	}
	bindTyped := func(name *ast.Ident, field *ast.Field) {
		bind(name, roleOther)
		if name.Name == "t" && isTestHandle(field.Type) {
			handles[name.Pos()] = true
		}
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			if typed.Tok == token.DEFINE {
				for _, target := range typed.Lhs {
					bind(target, roleOther)
				}
			}
		case *ast.DeclStmt:
			decl, ok := typed.Decl.(*ast.GenDecl)
			if !ok || decl.Tok == token.TYPE {
				return true
			}
			for _, spec := range decl.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range value.Names {
					if name.Name == "_" {
						continue
					}
					bind(name, roleOther)
				}
			}
		case *ast.RangeStmt:
			bind(typed.Key, roleIndex)
			bind(typed.Value, roleOther)
		case *ast.ForStmt:
			if init, ok := typed.Init.(*ast.AssignStmt); ok && init.Tok == token.DEFINE {
				for _, target := range init.Lhs {
					bind(target, roleIndex)
				}
			}
		case *ast.TypeSwitchStmt:
			if assign, ok := typed.Assign.(*ast.AssignStmt); ok {
				for _, target := range assign.Lhs {
					bind(target, roleOther)
				}
			}
		case *ast.FuncType:
			bindFields(typed.Params, bindTyped)
			bindFields(typed.Results, bindTyped)
		}
		return true
	})
	locals := make([]local, 0, len(order))
	for _, pos := range order {
		locals = append(locals, local{
			name:       names[pos],
			pos:        pos,
			role:       roles[pos],
			testHandle: handles[pos],
		})
	}
	return locals
}

// bindFields adds every named field of a parameter or result list, keeping the
// declared type so a test handle can be told from a one-letter local.
func bindFields(fields *ast.FieldList, bind func(*ast.Ident, *ast.Field)) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			bind(name, field)
		}
	}
}

// isTestHandle reports a parameter of the testing package, whose name the
// language and the standard library fix at one letter.
func isTestHandle(expr ast.Expr) bool {
	pointer, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "testing" {
		return false
	}
	return selector.Sel.Name == "T" || selector.Sel.Name == "B" ||
		selector.Sel.Name == "F" || selector.Sel.Name == "M"
}

// funcBody returns the block of a function declaration or literal, else nil.
func funcBody(node ast.Node) *ast.BlockStmt {
	switch typed := node.(type) {
	case *ast.FuncDecl:
		return typed.Body
	case *ast.FuncLit:
		return typed.Body
	}
	return nil
}

func startsUpper(name string) bool {
	first, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(first)
}

func isOneLetter(name string) bool {
	first, size := utf8.DecodeRuneInString(name)
	return size == len(name) && first != '_'
}
