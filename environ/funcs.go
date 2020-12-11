package environ

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gnorm.org/gnorm/run/data"

	"github.com/codemodus/kace"
	"github.com/jinzhu/inflection"
	"github.com/pkg/errors"
)

// FuncMap is the default list of functions available to templates.  If you add
// methods here, please keep them alphabetical.
var FuncMap = map[string]interface{}{
	"camel":        kace.Camel,
	"compare":      strings.Compare,
	"contains":     strings.Contains,
	"containsAny":  strings.ContainsAny,
	"count":        strings.Count,
	"dec":          dec,
	"equalFold":    strings.EqualFold,
	"fields":       strings.Fields,
	"hasPrefix":    strings.HasPrefix,
	"hasSuffix":    strings.HasSuffix,
	"inc":          inc,
	"strIndex":     strings.Index,
	"indexAny":     strings.IndexAny,
	"join":         strings.Join,
	"kebab":        kace.Kebab,
	"kebabUpper":   kace.KebabUpper,
	"lastIndex":    strings.LastIndex,
	"lastIndexAny": strings.LastIndexAny,
	"makeMap":      makeMap,
	"makeSlice":    makeSlice,
	"numbers":      numbers,
	"steps":        steps,
	"pascal":       kace.Pascal,
	"plural":       inflection.Plural,
	"repeat":       strings.Repeat,
	"replace":      strings.Replace,
	"singular":     inflection.Singular,
	"sliceString":  sliceString,
	"snake":        kace.Snake,
	"snakeUpper":   kace.SnakeUpper,
	"split":        strings.Split,
	"splitAfter":   strings.SplitAfter,
	"splitAfterN":  strings.SplitAfterN,
	"splitN":       strings.SplitN,
	"sub":          sub,
	"sum":          sum,
	"title":        strings.Title,
	"toLower":      strings.ToLower,
	"toTitle":      strings.ToTitle,
	"toUpper":      strings.ToUpper,
	"trim":         strings.Trim,
	"trimLeft":     strings.TrimLeft,
	"trimPrefix":   strings.TrimPrefix,
	"trimRight":    strings.TrimRight,
	"trimSpace":    strings.TrimSpace,
	"trimSuffix":   strings.TrimSuffix,
}

// sliceString returns a slice of s from index start to end.
func sliceString(s string, start, end int) string {
	return s[start:end]
}

// makeSlice returns the arguments as a single slice.  If all the arguments are
// strings, they are returned as a []string, otherwise they're returned as
// []interface{}.
func makeSlice(vals ...interface{}) interface{} {
	ss := make([]string, len(vals))
	for x := range vals {
		if s, ok := vals[x].(string); ok {
			ss[x] = s
		} else {
			// something was not a string, so just return the []interface{}
			return vals
		}
	}
	return ss
}

// makeMap expects an even number of parameters, in order to have name:value
// pairs.  All even values must be strings as keys.  Odd values may be any
// value. This is used to make maps to pass information into sub templates,
// range statements, etc.
func makeMap(vals ...interface{}) (map[string]interface{}, error) {
	if len(vals)%2 != 0 {
		return nil, errors.New("odd number of arguments passed to makeMap")
	}
	ret := make(map[string]interface{}, len(vals)/2)
	for x := 0; x < len(vals); x += 2 {
		s, ok := vals[x].(string)
		if !ok {
			return nil, fmt.Errorf("expected key values to be string, but got %T", vals[x])
		}
		ret[s] = vals[x+1]
	}
	return ret, nil
}

// dec decrements the argument's value by 1.
func dec(x int) int {
	return x - 1
}

// inc increments the argument's value by 1.
func inc(x int) int {
	return x + 1
}

// sum returns the sum of its arguments.
func sum(vals ...int) int {
	x := 0
	for _, v := range vals {
		x += v
	}
	return x
}

// sub subtracts the second and following values from the first argument.
func sub(x int, vals ...int) int {
	for _, v := range vals {
		x -= v
	}
	return x
}

// numbers returns a slice of strings of the numbers start to end (inclusive).
func numbers(start, end int) data.Strings {
	var s data.Strings
	for x := start; x <= end; x++ {
		s = append(s, strconv.Itoa(x))
	}
	return s
}

// steps returns a slice of strings of the numbers from start with length of len
func steps(start, len int) data.Strings {
	var s data.Strings
	for x := 0; x < len; x++ {
		s = append(s, strconv.Itoa(start+x))
	}
	return s
}

// Plugin returns a function which can be used in templates for executing plugins,
// dirs is the list of directories which are used fo plugin lookup.
func Plugin(dirs []string) func(string, string, interface{}) (interface{}, error) {
	return func(name, function string, ctx interface{}) (interface{}, error) {
		name, err := lookUpPlugin(dirs, name)
		if err != nil {
			return nil, err
		}
		return callPlugin(exec.Command, name, function, ctx)
	}
}

func lookUpPlugin(dirs []string, name string) (p string, err error) {
	for _, v := range dirs {
		p, err = exec.LookPath(filepath.Join(v, name))
		if err == nil {
			return
		}
	}
	return
}

func convert(v interface{}) interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		for k, v := range m {
			m[k] = convert(v)
		}
		return m
	}

	list, ok := v.([]interface{})
	if !ok {
		return v
	}

	var str []string
	for _, val := range list {
		s, ok := val.(string)
		if !ok {
			var out []interface{}
			for _, x := range list {
				out = append(out, convert(x))
			}
			return out
		}
		str = append(str, s)
	}
	return str
}

func callPlugin(runner cmdRunner, name, function string, ctx interface{}) (interface{}, error) {
	d := make(map[string]interface{})
	d["data"] = ctx
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	o, err := execJSON(runner, name, b, function)
	if err != nil {
		return nil, err
	}

	return convert(o["data"]), nil
}

type cmdRunner func(name string, args ...string) *exec.Cmd

// execJSON executes the plugin name with arguments args. It creates a io.Pipe
// to stdin of the command and writes the data into the pipe in a separate
// goroutine.
//
// The output is decoded as json data into a map[string]interface{}
func execJSON(runner cmdRunner, name string, data []byte, args ...string) (map[string]interface{}, error) {
	cmd := runner(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		defer stdin.Close()
		stdin.Write(data)
	}()
	v, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "error running plugin %q: ", string(v))
	}
	o := make(map[string]interface{})
	if err = json.Unmarshal(v, &o); err != nil {
		return nil, err
	}
	return o, nil
}
