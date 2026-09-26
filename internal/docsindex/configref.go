package docsindex

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// This file turns the configuration reference (docs/configuration.md) from a
// page that merely names every ARMOR_* variable into one whose entries are
// checked against the code: ParseConfigRefRows parses the reference's tables,
// CodeDefaults and ValidationRules extract what internal/config actually
// does, and ConfigRefProblems reports every place the two disagree.

// CodeDefault is a default value internal/config declares mechanically, as
// extracted from the source: the second argument of a getEnv/getEnvInt call,
// or a bool env flag compared against "true", either inline or through the
// local the env read was assigned to.
type CodeDefault struct {
	Var   string // the ARMOR_* variable name
	Kind  string // "string", "int" or "bool"
	Value string // the default as the reference must state it
}

// ValidationRule is a constraint internal/config enforces on a variable,
// extracted from its validation error messages. Tokens are the anchors the
// reference must carry for the rule: bound numbers ("4096", "32 bytes") and
// quoted accepted values ("'b2'"); a rule with no tokens is dropped, since
// there is nothing mechanical to check the prose against.
type ValidationRule struct {
	Var    string   // the variable the rule constrains (a family name ends in "_")
	Tokens []string // anchors that must appear in the variable's reference row
}

// ConfigRefRow is one data row of a configuration-reference table.
type ConfigRefRow struct {
	Line     int      // 1-based line of the row in the reference document
	Vars     []string // ARMOR_* names the Variable cell claims (families end in "_")
	Required string   // Required cell
	Default  string   // Default cell
	Desc     string   // Description cell — the variable's operational effect
}

var (
	// goStringBody matches the inside of a double-quoted Go string literal,
	// escapes included.
	goStringBody = `(?:[^"\\]|\\.)*`
	// envStringDefault matches getEnv("ARMOR_X", "default").
	envStringDefault = regexp.MustCompile(`getEnv\(\s*"(ARMOR_[A-Z0-9_]+)"\s*,\s*"(` + goStringBody + `)"\s*\)`)
	// envIntDefault matches getEnvInt("ARMOR_X", 100).
	envIntDefault = regexp.MustCompile(`getEnvInt\(\s*"(ARMOR_[A-Z0-9_]+)"\s*,\s*(\d+)\s*\)`)
	// envBoolFlag matches os.Getenv("ARMOR_X") == "true".
	envBoolFlag = regexp.MustCompile(`os\.Getenv\(\s*"(ARMOR_[A-Z0-9_]+)"\s*\)\s*==\s*"true"`)
	// envBoolAssign matches `name := os.Getenv("ARMOR_X")` — the first half of
	// the two-line boolean idiom whose "true" comparison happens on the named
	// local rather than on the call itself.
	envBoolAssign = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*os\.Getenv\(\s*"(ARMOR_[A-Z0-9_]+)"\s*\)`)
	// envTrueCmp marks a local compared against "true".
	envTrueCmp = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*==\s*"true"`)
	// goStringLiteral matches a whole double-quoted Go string literal.
	goStringLiteral = regexp.MustCompile(`"` + goStringBody + `"`)
	// ruleWords marks an error literal as stating a constraint on a variable.
	ruleWords = regexp.MustCompile(`must be|is required|is reserved|must not|must stay`)
	// ruleKey extracts the variable a rule literal names, stopping at any
	// formatting placeholder (ARMOR_MEK_%s -> ARMOR_MEK_).
	ruleKey = regexp.MustCompile(`ARMOR_[A-Z0-9_]*`)
	// ruleNumber finds numeric bounds in a rule; ruleAlternation marks the
	// "2 or 3" enumerations whose single-digit members are significant.
	ruleNumber      = regexp.MustCompile(`\d+`)
	ruleAlternation = regexp.MustCompile(`\b\d+(?:\s+or\s+\d+)+\b`)
	// ruleQuoted finds single-quoted accepted values in a rule.
	ruleQuoted = regexp.MustCompile(`'([^']+)'`)
	// envVarName extracts bare ARMOR_* names from a table's Variable cell.
	envVarName = regexp.MustCompile(`ARMOR_[A-Z0-9_]*`)
	// ruleTruncators end a rule statement; anything after them is the
	// observed value of the error ("... , got '%s'"), not part of the rule.
	ruleTruncators = []string{", got", ": %w", ": %v"}
)

// forEachConfigSource runs visit over every non-test .go file directly in
// configDir (the config package is flat), in lexical order.
func forEachConfigSource(configDir string, visit func(src []byte) error) error {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.Type().IsRegular() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	for _, name := range files {
		src, err := os.ReadFile(filepath.Join(configDir, name))
		if err != nil {
			return err
		}
		if err := visit(src); err != nil {
			return err
		}
	}
	return nil
}

// CodeDefaults extracts, sorted by variable, every default internal/config
// declares in a mechanically recognizable way. Variables whose default is
// computed (derived from the hostname, a tri-state string, ...) are simply
// absent: parity is enforced only where the code itself states a literal.
func CodeDefaults(configDir string) ([]CodeDefault, error) {
	seen := make(map[string]CodeDefault)
	err := forEachConfigSource(configDir, func(src []byte) error {
		text := string(src)
		for _, m := range envStringDefault.FindAllStringSubmatch(text, -1) {
			seen[m[1]] = CodeDefault{Var: m[1], Kind: "string", Value: m[2]}
		}
		for _, m := range envIntDefault.FindAllStringSubmatch(text, -1) {
			seen[m[1]] = CodeDefault{Var: m[1], Kind: "int", Value: m[2]}
		}
		for _, m := range envBoolFlag.FindAllStringSubmatch(text, -1) {
			seen[m[1]] = CodeDefault{Var: m[1], Kind: "bool", Value: "false"}
		}
		// The two-line boolean idiom: the local assigned from the env read is
		// the thing later compared against "true". Both halves must be in the
		// same file for the pairing to be trusted.
		assigned := make(map[string]string)
		compared := make(map[string]bool)
		for _, m := range envBoolAssign.FindAllStringSubmatch(text, -1) {
			assigned[m[1]] = m[2]
		}
		for _, m := range envTrueCmp.FindAllStringSubmatch(text, -1) {
			compared[m[1]] = true
		}
		for local, v := range assigned {
			if compared[local] {
				seen[v] = CodeDefault{Var: v, Kind: "bool", Value: "false"}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	defaults := make([]CodeDefault, 0, len(seen))
	for _, d := range seen {
		defaults = append(defaults, d)
	}
	sort.Slice(defaults, func(i, j int) bool { return defaults[i].Var < defaults[j].Var })
	return defaults, nil
}

// ValidationRules extracts, sorted by variable, the checkable constraints
// internal/config's validation errors state. Rules whose message carries no
// anchor token (pure prose like "must be a relative path") are dropped:
// there is nothing mechanical to hold the reference to.
func ValidationRules(configDir string) ([]ValidationRule, error) {
	tokensByKey := make(map[string]map[string]bool)
	err := forEachConfigSource(configDir, func(src []byte) error {
		for _, lit := range goStringLiteral.FindAllString(string(src), -1) {
			unquoted, uerr := strconv.Unquote(lit)
			if uerr != nil {
				unquoted = lit[1 : len(lit)-1]
			}
			if !strings.Contains(unquoted, "ARMOR_") || !ruleWords.MatchString(unquoted) {
				continue
			}
			ruleText := unquoted
			for _, t := range ruleTruncators {
				if i := strings.Index(ruleText, t); i >= 0 {
					ruleText = ruleText[:i]
				}
			}
			tokens := ruleTokens(ruleText)
			if len(tokens) == 0 {
				continue
			}
			key := ruleKey.FindString(unquoted)
			if tokensByKey[key] == nil {
				tokensByKey[key] = make(map[string]bool)
			}
			for _, t := range tokens {
				tokensByKey[key][t] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	rules := make([]ValidationRule, 0, len(tokensByKey))
	for key, toks := range tokensByKey {
		tokens := make([]string, 0, len(toks))
		for t := range toks {
			tokens = append(tokens, t)
		}
		sort.Strings(tokens)
		rules = append(rules, ValidationRule{Var: key, Tokens: tokens})
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].Var < rules[j].Var })
	return rules, nil
}

// ruleTokens returns the anchor tokens a rule statement requires the
// reference to carry: quoted accepted values and numeric bounds. Single-digit
// numbers count only as enumerations ("2 or 3") — a bare "at least 1" or the
// "power of 2" phrasing would otherwise force meaningless matches.
func ruleTokens(ruleText string) []string {
	seen := make(map[string]bool)
	var tokens []string
	add := func(t string) {
		if t != "" && !seen[t] {
			seen[t] = true
			tokens = append(tokens, t)
		}
	}
	for _, m := range ruleQuoted.FindAllStringSubmatch(ruleText, -1) {
		if !strings.Contains(m[1], "%") {
			add(m[1])
		}
	}
	singles := make(map[string]bool)
	for _, alt := range ruleAlternation.FindAllString(ruleText, -1) {
		for _, n := range ruleNumber.FindAllString(alt, -1) {
			singles[n] = true
		}
	}
	for _, n := range ruleNumber.FindAllString(ruleText, -1) {
		if len(n) > 1 || singles[n] {
			add(n)
		}
	}
	sort.Strings(tokens)
	return tokens
}

// splitCells splits one markdown table row into cells, treating '|'
// characters inside backtick spans as text. Cells are trimmed; the empty
// cells the row's edge separators produce are dropped.
func splitCells(line string) []string {
	var cells []string
	var buf strings.Builder
	inCode := false
	flush := func() {
		cells = append(cells, strings.TrimSpace(buf.String()))
		buf.Reset()
	}
	for _, r := range line {
		switch {
		case r == '`':
			inCode = !inCode
			buf.WriteRune(r)
		case r == '|' && !inCode:
			flush()
		default:
			buf.WriteRune(r)
		}
	}
	flush()
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// isSeparatorRow reports whether a table line is the dashed column ruler.
func isSeparatorRow(line string) bool {
	cells := splitCells(line)
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.Trim(c, ":")
		if c == "" || strings.Trim(c, "-") != "" {
			return false
		}
	}
	return true
}

// ParseConfigRefRows parses every table of a configuration-reference document
// whose header starts with a Variable column and carries Default and
// Description columns, in document order. Prose, non-reference tables and the
// column ruler are skipped; the header's column order is honored.
func ParseConfigRefRows(doc []byte) []ConfigRefRow {
	lines := strings.Split(string(doc), "\n")
	var rows []ConfigRefRow
	for i := 0; i < len(lines); {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
			i++
			continue
		}
		start := i
		var block []string
		for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
			block = append(block, lines[i])
		}
		if len(block) < 3 || !isSeparatorRow(block[1]) {
			continue
		}
		header := splitCells(block[0])
		if len(header) == 0 || header[0] != "Variable" {
			continue
		}
		defIdx, descIdx, reqIdx := -1, -1, -1
		for j, h := range header {
			switch h {
			case "Default":
				defIdx = j
			case "Description":
				descIdx = j
			case "Required":
				reqIdx = j
			}
		}
		if defIdx < 0 || descIdx < 0 {
			continue
		}
		for k := 2; k < len(block); k++ {
			cells := splitCells(block[k])
			cell := func(idx int) string {
				if idx >= 0 && idx < len(cells) {
					return cells[idx]
				}
				return ""
			}
			seen := make(map[string]bool)
			var vars []string
			for _, v := range envVarName.FindAllString(cell(0), -1) {
				if !seen[v] {
					seen[v] = true
					vars = append(vars, v)
				}
			}
			rows = append(rows, ConfigRefRow{
				Line:     start + k + 1,
				Vars:     vars,
				Required: cell(reqIdx),
				Default:  cell(defIdx),
				Desc:     cell(descIdx),
			})
		}
	}
	return rows
}

// docText normalizes a cell for token matching: code formatting is not
// meaningful to the parity check, only the text it wraps.
func docText(s string) string {
	return strings.ReplaceAll(s, "`", "")
}

// ConfigRefProblems reports every way a configuration reference fails to
// document vars with their defaults and validation rules, sorted:
//
//   - a variable read by the code with no reference table row at all;
//   - a non-family variable spread over several rows;
//   - a row claiming a variable with an empty Required, Default or
//     Description cell;
//   - a row's Default that disagrees with the default the code declares;
//   - a variable's row missing an anchor its validation rule requires;
//   - a validation rule naming a variable the code never reads.
//
// Rows claiming no variable from vars are left alone: they document
// variables other packages read, which this package does not check.
func ConfigRefProblems(vars []string, defaults []CodeDefault, rules []ValidationRule, rows []ConfigRefRow) []string {
	// claims maps each variable to the rows whose Variable cell names it;
	// claimedBy is the per-row inverse.
	claims := make(map[string][]int)
	claimedBy := make([][]string, len(rows))
	for ri, row := range rows {
		for _, tok := range row.Vars {
			for _, v := range vars {
				if tok == v || (strings.HasSuffix(v, "_") && strings.HasPrefix(tok, v)) {
					claims[v] = append(claims[v], ri)
					claimedBy[ri] = append(claimedBy[ri], v)
				}
			}
		}
	}
	// bestRow picks the row of a variable's claims the variable's own
	// documentation lives on: an exact name over a family placeholder, among
	// placeholders the shortest (ARMOR_MEK_<NAME> over ARMOR_MEK_<NAME>_RING),
	// then the earliest.
	bestRow := func(v string) (ConfigRefRow, bool) {
		idx := claims[v]
		if len(idx) == 0 {
			return ConfigRefRow{}, false
		}
		best := idx[0]
		for _, i := range idx[1:] {
			switch {
			case len(bestVarToken(rows[i], v)) < len(bestVarToken(rows[best], v)):
				best = i
			case len(bestVarToken(rows[i], v)) == len(bestVarToken(rows[best], v)) && rows[i].Line < rows[best].Line:
				best = i
			}
		}
		return rows[best], true
	}

	var problems []string
	for _, v := range vars {
		idx, ok := claims[v]
		if !ok {
			problems = append(problems, fmt.Sprintf("configuration reference has no table row for %s", v))
			continue
		}
		if !strings.HasSuffix(v, "_") && len(idx) > 1 {
			ls := make([]string, 0, len(idx))
			for _, i := range idx {
				ls = append(ls, strconv.Itoa(rows[i].Line))
			}
			problems = append(problems, fmt.Sprintf(
				"configuration reference documents %s in %d rows (lines %s)", v, len(idx), strings.Join(ls, ", ")))
		}
	}

	for _, d := range defaults {
		row, ok := bestRow(d.Var)
		if !ok {
			continue // already reported as missing coverage
		}
		if got := strings.TrimSpace(docText(row.Default)); got != d.Value {
			problems = append(problems, fmt.Sprintf(
				"configuration reference line %d states default %q for %s; internal/config declares %s default %q",
				row.Line, got, d.Var, d.Kind, d.Value))
		}
	}

	for _, rule := range rules {
		v := resolveRuleVar(rule.Var, vars)
		if v == "" {
			problems = append(problems, fmt.Sprintf(
				"validation rule names %s, which internal/config does not read", rule.Var))
			continue
		}
		row, ok := bestRow(v)
		if !ok {
			continue // already reported as missing coverage
		}
		text := docText(row.Default + " " + row.Desc)
		var missing []string
		for _, t := range rule.Tokens {
			if !strings.Contains(text, t) {
				missing = append(missing, t)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf(
				"configuration reference line %d does not state the validation rule internal/config enforces on %s (missing %s)",
				row.Line, v, strings.Join(missing, ", ")))
		}
	}

	for ri, row := range rows {
		if len(claimedBy[ri]) == 0 {
			continue // documents variables outside internal/config
		}
		for _, c := range []struct {
			name, val string
		}{{"Required", row.Required}, {"Default", row.Default}, {"Description", row.Desc}} {
			if strings.TrimSpace(c.val) == "" {
				problems = append(problems, fmt.Sprintf(
					"configuration reference line %d has an empty %s cell", row.Line, c.name))
			}
		}
	}

	sort.Strings(problems)
	return problems
}

// bestVarToken returns the token of a row's Variable cell that claims v:
// the exact name if present, else the shortest family-placeholder match.
func bestVarToken(row ConfigRefRow, v string) string {
	best := ""
	for _, tok := range row.Vars {
		if tok == v {
			return tok
		}
		if strings.HasSuffix(v, "_") && strings.HasPrefix(tok, v) {
			if best == "" || len(tok) < len(best) {
				best = tok
			}
		}
	}
	return best
}

// resolveRuleVar maps a rule's key (possibly a placeholder-bearing literal
// like ARMOR_MEK_DEFAULT) to the variable from vars it constrains: exact
// match, a family name, or the family the key belongs to.
func resolveRuleVar(key string, vars []string) string {
	for _, v := range vars {
		if v == key {
			return v
		}
	}
	for _, v := range vars {
		if strings.HasSuffix(v, "_") && strings.HasPrefix(key, v) {
			return v
		}
	}
	return ""
}
