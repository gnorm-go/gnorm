package run

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"text/template"

	"gnorm.org/gnorm/environ"
)

func TestAtomicGenerate(t *testing.T) {
	target := OutputTarget{
		Filename: template.Must(template.New("").Parse("{{.}}")),
		// the contents tempalte will fail to execute becase the contents will
		// not have a .Name field.
		Contents: template.Must(template.New("").Parse("{{.Name}}")),
	}
	env := environ.Values{
		Log: log.New(ioutil.Discard, "", 0),
	}
	filename := "testfile.out"
	original := []byte("goodbye world")
	err := ioutil.WriteFile(filename, original, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filename)
	contents := "hello world"
	err = genFile(env, filename, contents, target, nil)
	if err == nil {
		t.Fatal("Unexpected nil error generating contents. Should have failed.")
	}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, original) {
		t.Fatalf("Expected file to be unchanged, but was different.  Expected: %q, got: %q", original, b)
	}
}

func TestCopyStaticFiles(t *testing.T) {
	originPaths := []string{
		"base/base.md",
		"base/level_one/level_one.md",
		"base/level_two/level_two.md",
	}
	source := "testdata"
	dest := "static_asset"

	err := copyStaticFiles(environ.Values{}, source, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(dest)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// make sure the structure is preserved
	var newPaths []string
	filepath.Walk(dest, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		r, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		newPaths = append(newPaths, r)
		return nil
	})

	sort.Strings(originPaths)
	sort.Strings(newPaths)

	if !reflect.DeepEqual(originPaths, newPaths) {
		t.Errorf("expected %v to equal %v", newPaths, originPaths)
	}
}
