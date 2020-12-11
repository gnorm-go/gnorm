package environ

import "strings"

func strFuncs() strfunc {
	return strfunc{}
}

type strfunc struct{}

func (strfunc) Compare(a, b string) int {
	return strings.Compare(a, b)
}

func (strfunc) Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func (strfunc) ContainsAny(s, chars string) bool {
	return strings.ContainsAny(s, chars)
}

func (strfunc) Count(s, substr string) int {
	return strings.Count(s, substr)
}

func (strfunc) EqualFold(s, t string) bool {
	return strings.EqualFold(s, t)
}

func (strfunc) Fields(s string) []string {
	return strings.Fields(s)
}

func (strfunc) HasPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func (strfunc) Index(s, substr string) int {
	return strings.Index(s, substr)
}

func (strfunc) IndexAny(s, chars string) int {
	return strings.IndexAny(s, chars)
}

func (strfunc) Join(a []string, sep string) string {
	return strings.Join(a, sep)
}

func (strfunc) LastIndex(s, substr string) int {
	return strings.LastIndex(s, substr)
}

func (strfunc) LastIndexAny(s, chars string) int {
	return strings.LastIndexAny(s, chars)
}

func (strfunc) Repeat(s string, count int) string {
	return strings.Repeat(s, count)
}

func (strfunc) Replace(s, old, new string, n int) string {
	return strings.Replace(s, old, new, n)
}

func (strfunc) Split(s, sep string) []string {
	return strings.Split(s, sep)
}

func (strfunc) SplitAfter(s, sep string) []string {
	return strings.SplitAfter(s, sep)
}

func (strfunc) SplitAfterN(s, sep string, n int) []string {
	return strings.SplitAfterN(s, sep, n)
}

func (strfunc) SplitN(s, sep string, n int) []string {
	return strings.SplitN(s, sep, n)
}

func (strfunc) Title(s string) string {
	return strings.Title(s)
}

func (strfunc) ToLower(s string) string {
	return strings.ToLower(s)
}

func (strfunc) ToTitle(s string) string {
	return strings.ToTitle(s)
}

func (strfunc) ToUpper(s string) string {
	return strings.ToUpper(s)
}

func (strfunc) Trim(s string, cutset string) string {
	return strings.Trim(s, cutset)
}

func (strfunc) TrimLeft(s string, cutset string) string {
	return strings.TrimLeft(s, cutset)
}

func (strfunc) TrimPrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}

func (strfunc) TrimRight(s string, cutset string) string {
	return strings.TrimRight(s, cutset)
}

func (strfunc) TrimSpace(s string) string {
	return strings.TrimSpace(s)
}

func (strfunc) TrimSuffix(s, suffix string) string {
	return strings.TrimSuffix(s, suffix)
}
