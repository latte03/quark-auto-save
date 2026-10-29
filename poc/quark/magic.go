package quark

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dlclark/regexp2"
)

// Rule is one named magic-regex preset. The id is stable: tasks reference it,
// so renaming is always safe.
type Rule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
}

type Magic struct {
	rules    map[string]Rule
	byID     map[string]Rule
	cache    map[string]*regexp2.Regexp
	vars     map[string][]string
	varOrder []string
	taskName string
	dir      map[int]string
	now      func() time.Time
}

// Mirrors MagicRename.magic_variable order; the order decides which key
// wins when a replacement contains several of them.
var magicVarPatterns = []struct {
	key      string
	patterns []string
}{
	{"{TASKNAME}", nil},
	{"{I}", nil},
	{"{EXT}", []string{`(?<=\.)\w+$`}},
	{"{CHINESE}", []string{`[\u4e00-\u9fa5]{2,}`}},
	{"{DATE}", []string{
		`(18|19|20)?\d{2}[\.\-/年]\d{1,2}[\.\-/月]\d{1,2}`,
		`(?<!\d)[12]\d{3}[01]?\d[0123]?\d`,
		`(?<!\d)[01]?\d[\.\-/月][0123]?\d`,
	}},
	{"{YEAR}", []string{`(?<!\d)(18|19|20)\d{2}(?!\d)`}},
	{"{S}", []string{`(?<=[Ss])\d{1,2}(?=[EeXx])`, `(?<=[Ss])\d{1,2}`}},
	{"{SXX}", []string{`[Ss]\d{1,2}(?=[EeXx])`, `[Ss]\d{1,2}`}},
	{"{E}", []string{
		`(?<=[Ss]\d\d[Ee])\d{1,3}`,
		`(?<=[Ee])\d{1,3}`,
		`(?<=[Ee][Pp])\d{1,3}`,
		`(?<=第)\d{1,3}(?=[集期话部篇])`,
		`(?<!\d)\d{1,3}(?=[集期话部篇])`,
		`(?!.*19)(?!.*20)(?<=[\._])\d{1,3}(?=[\._])`,
		`^\d{1,3}(?=\.\w+)`,
		`(?<!\d)\d{1,3}(?!\d)(?!$)`,
	}},
	{"{PART}", []string{
		`(?<=[集期话部篇第])[上中下一二三四五六七八九十]`,
		`[上中下一二三四五六七八九十]`,
	}},
	{"{VER}", []string{`[\u4e00-\u9fa5]+版`}},
}

var magicPriority = []string{
	"上", "中", "下", "一", "二", "三", "四", "五", "六", "七", "八", "九", "十", "百", "千", "万",
}

func NewMagic(rules []Rule) *Magic {
	m := &Magic{
		rules: make(map[string]Rule, len(rules)),
		byID:  make(map[string]Rule, len(rules)),
		cache: map[string]*regexp2.Regexp{},
		vars:  map[string][]string{},
		dir:   map[int]string{},
		now:   time.Now,
	}
	for _, rule := range rules {
		if rule.ID != "" {
			m.byID[rule.ID] = rule
		}
		if rule.Name != "" {
			m.rules[rule.Name] = rule
		}
	}
	for _, item := range magicVarPatterns {
		m.vars[item.key] = item.patterns
		m.varOrder = append(m.varOrder, item.key)
	}
	return m
}

// ResolveTask prefers the stable rule id and only falls back to a name or a
// raw regex for tasks that were never migrated. An id that no longer exists
// fails closed so a deleted rule cannot turn into "match everything".
func (m *Magic) ResolveTask(magicID, pattern, replace string) (string, string) {
	if magicID != "" {
		if rule, ok := m.byID[magicID]; ok {
			if replace == "" {
				replace = rule.Replace
			}
			return rule.Pattern, replace
		}
		return `(?!x)x`, replace
	}
	return m.Resolve(pattern, replace)
}

// SetClock keeps {DATE} deterministic in tests.
func (m *Magic) SetClock(now func() time.Time) { m.now = now }

func (m *Magic) SetTaskName(name string) { m.taskName = name }

func (m *Magic) DirFilenameDict() map[int]string {
	out := make(map[int]string, len(m.dir))
	for k, v := range m.dir {
		out[k] = v
	}
	return out
}

func (m *Magic) Resolve(key, replace string) (pattern, repl string) {
	if rule, ok := m.rules[key]; ok {
		pattern = rule.Pattern
		if replace == "" {
			replace = rule.Replace
		}
	} else {
		pattern = key
	}
	return pattern, replace
}

func (m *Magic) Match(pattern, filename string) (bool, error) {
	re, err := m.compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(filename)
}

func (m *Magic) Sub(pattern, replace, filename string) (string, error) {
	if replace == "" {
		return filename, nil
	}
	for _, key := range m.varOrder {
		if !strings.Contains(replace, key) {
			continue
		}
		patterns := m.vars[key]
		matched := ""
		for _, p := range patterns {
			value, ok, err := m.search(p, filename)
			if err != nil {
				return "", err
			}
			if !ok {
				continue
			}
			if key == "{DATE}" {
				value = m.fillYearDigits(value)
			}
			replace = strings.ReplaceAll(replace, key, value)
			matched = value
			break
		}
		switch {
		case key == "{TASKNAME}":
			replace = strings.ReplaceAll(replace, key, m.taskName)
		case key == "{SXX}" && matched == "":
			replace = strings.ReplaceAll(replace, key, "S01")
		case key == "{I}":
			// Resolved later by SortFileList.
		case len(patterns) > 0:
			replace = strings.ReplaceAll(replace, key, "")
		}
	}
	if pattern != "" && replace != "" {
		re, err := m.compile(pattern)
		if err != nil {
			return "", err
		}
		return re.Replace(filename, pyReplacementToDotNet(replace), -1, -1)
	}
	return replace, nil
}

func (m *Magic) search(pattern, filename string) (string, bool, error) {
	re, err := m.compile(pattern)
	if err != nil {
		return "", false, err
	}
	match, err := re.FindStringMatch(filename)
	if err != nil {
		return "", false, err
	}
	if match == nil {
		return "", false, nil
	}
	return match.String(), true, nil
}

// fillYearDigits mirrors the {DATE} post-processing: keep digits only and
// prefix the missing leading digits with the current year.
func (m *Magic) fillYearDigits(value string) string {
	var digits []rune
	for _, r := range value {
		if unicode.IsDigit(r) {
			digits = append(digits, r)
		}
	}
	value = string(digits)
	year := strconv.Itoa(m.now().Year())
	stop := 8 - len(value)
	if stop < 0 {
		stop = len(year) + stop
	}
	if stop > len(year) {
		stop = len(year)
	}
	if stop < 0 {
		stop = 0
	}
	return year[:stop] + value
}

func (m *Magic) compile(pattern string) (*regexp2.Regexp, error) {
	if re, ok := m.cache[pattern]; ok {
		return re, nil
	}
	re, err := compileMagic(pattern)
	if err != nil {
		return nil, err
	}
	m.cache[pattern] = re
	return re, nil
}

// Exists mirrors MagicRename.is_exists: it returns the colliding name, or "".
func (m *Magic) Exists(filename string, filenames []string, ignoreExt bool) string {
	if ignoreExt {
		filename = splitExt(filename)
		filenames = mapSlice(filenames, splitExt)
	}
	if match := reIPlus.FindString(filename); match != "" {
		patternI := strings.Repeat(`\d`, strings.Count(match, "I"))
		pattern := strings.ReplaceAll(pyEscape(filename), pyEscape(match), patternI)
		re, err := m.compile("^(?:" + pattern + ")")
		if err != nil {
			return ""
		}
		for _, name := range filenames {
			ok, err := re.MatchString(name)
			if err == nil && ok {
				return name
			}
		}
		return ""
	}
	for _, name := range filenames {
		if name == filename {
			return name
		}
	}
	return ""
}

// SetDirFileList mirrors set_dir_file_list: it reads the numbering already
// present in the destination so {I+} continues instead of restarting.
func (m *Magic) SetDirFileList(files []ShareFile, replace string) error {
	m.dir = map[int]string{}
	var names []string
	for _, f := range files {
		if !f.Dir {
			names = append(names, f.FileName)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}
	magicI := reIPlus.FindString(replace)
	if magicI == "" {
		return nil
	}
	patternI := strings.Repeat(`\d`, strings.Count(magicI, "I"))
	pattern := strings.ReplaceAll(replace, magicI, "🔢")
	for _, key := range m.varOrder {
		pattern = strings.ReplaceAll(pattern, key, "🔣")
	}
	pattern = reBackref.ReplaceAllString(pattern, "🔣")
	escaped := pyEscape(pattern)
	escaped = strings.ReplaceAll(escaped, "🔣", ".*?")
	escaped = strings.ReplaceAll(escaped, "🔢", ")("+patternI+")(")
	full := "^(?:" + "(" + escaped + ")" + ")"
	re, err := m.compile(full)
	if err != nil {
		return err
	}
	groups := func(name string) (string, int, string, bool) {
		match, err := re.FindStringMatch(name)
		if err != nil || match == nil {
			return "", 0, "", false
		}
		g1 := match.GroupByNumber(1)
		g2 := match.GroupByNumber(2)
		g3 := match.GroupByNumber(3)
		if g1 == nil || g2 == nil || g3 == nil {
			return "", 0, "", false
		}
		n, err := strconv.Atoi(g2.String())
		if err != nil {
			return "", 0, "", false
		}
		return g1.String(), n, g3.String(), true
	}
	for _, name := range names {
		if g1, n, g3, ok := groups(name); ok {
			m.dir[n] = g1 + magicI + g3
		}
	}
	return nil
}

// SortFileList mirrors sort_file_list: it merges the destination names with
// the incoming ones, sorts naturally and fills in {I+} placeholders.
func (m *Magic) SortFileList(files []*ShareFile) {
	var keys []string
	seen := map[string]bool{}
	add := func(key string) {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for _, f := range files {
		if f.FileNameRe != "" && !f.Dir {
			add(f.FileNameRe + "_" + strconv.FormatInt(f.UpdatedAt, 10))
		}
	}
	dirNumbers := make([]int, 0, len(m.dir))
	for n := range m.dir {
		dirNumbers = append(dirNumbers, n)
	}
	sort.Ints(dirNumbers)
	dirValues := map[string]bool{}
	for _, n := range dirNumbers {
		dirValues[m.dir[n]] = true
		add(m.dir[n])
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return naturalLess(sortKey(keys[i]), sortKey(keys[j]))
	})
	index := map[string]int{}
	for pos, name := range keys {
		if dirValues[name] {
			continue
		}
		i := pos + 1
		for {
			if _, taken := m.dir[i]; !taken {
				break
			}
			i++
		}
		m.dir[i] = name
		index[name] = i
	}
	for _, f := range files {
		if f.FileNameRe == "" {
			continue
		}
		magicI := reIPlus.FindString(f.FileNameRe)
		if magicI == "" {
			continue
		}
		i := index[f.FileNameRe+"_"+strconv.FormatInt(f.UpdatedAt, 10)]
		f.FileNameRe = strings.ReplaceAll(f.FileNameRe, magicI, fmt.Sprintf("%0*d", strings.Count(magicI, "I"), i))
	}
}

var (
	reIPlus   = regexp.MustCompile(`\{I+\}`)
	reBackref = regexp.MustCompile(`\\[0-9]+`)
)

func sortKey(name string) string {
	for i, keyword := range magicPriority {
		if strings.Contains(name, keyword) {
			name = strings.ReplaceAll(name, keyword, fmt.Sprintf("_%02d_", i))
		}
	}
	return name
}

// naturalLess approximates natsort's default (INT, case-sensitive) order.
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		aDigit := isDigit(a[0])
		bDigit := isDigit(b[0])
		if aDigit && bDigit {
			ai, alen := leadingNumber(a)
			bi, blen := leadingNumber(b)
			if c := compareNumericStrings(ai, bi); c != 0 {
				return c < 0
			}
			a = a[alen:]
			b = b[blen:]
			continue
		}
		if aDigit != bDigit {
			// natsort orders numbers before text at the same position.
			return aDigit
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a = a[1:]
		b = b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func leadingNumber(s string) (string, int) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], i
}

func compareNumericStrings(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

// pyEscape matches Python 3.7+ re.escape, which only escapes these chars.
func pyEscape(s string) string {
	const specials = `()[]{}?*+-|^$\.&~# `
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch {
		case strings.ContainsRune(specials, r), r == '\t', r == '\n', r == '\r', r == '\v', r == '\f':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitExt mirrors os.path.splitext: leading dots belong to the root name.
func splitExt(name string) string {
	sep := strings.LastIndexByte(name, '/')
	dot := strings.LastIndexByte(name, '.')
	if dot <= sep {
		return name
	}
	for i := sep + 1; i < dot; i++ {
		if name[i] != '.' {
			return name[:dot]
		}
	}
	return name
}

func mapSlice(in []string, fn func(string) string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

func compileMagic(pattern string) (*regexp2.Regexp, error) {
	re, err := regexp2.Compile(pyPatternToDotNet(pattern), regexp2.None)
	if err != nil {
		return nil, fmt.Errorf("compile pattern: %w", err)
	}
	re.MatchTimeout = 2 * time.Second
	return re, nil
}

// Python accepts a backslash before any non-alphanumeric as a literal;
// .NET rejects several of those (e.g. \_). Keep the escapes that mean
// something and unwrap the rest.
func pyPatternToDotNet(pattern string) string {
	pattern = strings.ReplaceAll(pattern, "(?P<", "(?<")
	var b strings.Builder
	b.Grow(len(pattern))
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c != '\\' || i+1 >= len(pattern) {
			b.WriteByte(c)
			continue
		}
		next := pattern[i+1]
		if keepsEscape(next) {
			b.WriteByte('\\')
		}
		b.WriteByte(next)
		i++
	}
	return b.String()
}

func keepsEscape(c byte) bool {
	if c >= '0' && c <= '9' {
		return true
	}
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
		return true
	}
	return strings.ContainsRune(`.^$*+?()[]{}|\-/ `, rune(c))
}

// Python replacements use \1 for groups and treat $ literally; regexp2
// follows .NET, where $1 is a group and a literal $ must be doubled.
func pyReplacementToDotNet(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			if i+1 >= len(s) {
				b.WriteString(`\\`)
				continue
			}
			n := s[i+1]
			switch {
			case n >= '0' && n <= '9':
				b.WriteByte('$')
				b.WriteByte(n)
			case n == '\\':
				b.WriteByte('\\')
			case n == 'n':
				b.WriteByte('\n')
			case n == 't':
				b.WriteByte('\t')
			case n == 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(n)
			}
			i++
		case '$':
			b.WriteString("$$")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
