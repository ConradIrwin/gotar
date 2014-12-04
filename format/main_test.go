package gotar

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

type TMonad struct {
	*testing.T
}

func (t TMonad) FatalIf(err error) {
	if err != nil {
		panic(err)
	}
}

type metaData struct {
	Name string
	UUID string
}

var CODE = "#!ruby\nputs 'hello world'\n"
var TEXT = "testing testing 1 2 3"

func TestEndToEnd(_t *testing.T) {
	t := TMonad{_t}

	f, err := ioutil.TempFile("", "")
	t.FatalIf(err)

	g, err := ioutil.TempFile("/tmp", "fff_")
	t.FatalIf(err)

	_, err = g.Write([]byte(TEXT))
	t.FatalIf(err)

	err = g.Close()
	t.FatalIf(err)

	app := NewWriter(f)
	err = app.WriteApp(bytes.NewBuffer([]byte(CODE)))
	t.FatalIf(err)

	err = app.WriteFile(g, "/tmp")
	t.FatalIf(err)

	app.WriteMetaData(metaData{"foo", "bar"})

	app.Close()

	err = f.Close()
	t.FatalIf(err)

	f, err = os.Open(f.Name())
	t.FatalIf(err)

	a, err := Read(f)
	t.FatalIf(err)

	code, err := ioutil.ReadAll(a.ReadApp())
	t.FatalIf(err)

	if string(code) != CODE {
		t.Errorf("App got %s, not %s", code, CODE)
	}

	hdr, contents, err := a.NextFile()
	t.FatalIf(err)

	expected := strings.TrimPrefix(g.Name(), "/tmp/")
	if hdr.Name != expected {
		t.Errorf("File name got %s, not %s", hdr.Name, expected)
	}

	if string(contents) != TEXT {
		t.Errorf("Contents got %s, not %s", contents, TEXT)
	}

	var m metaData
	err = a.ReadMetaData(&m)
	t.FatalIf(err)

	if m.Name != "foo" || m.UUID != "bar" {
		t.Errorf("Couldn't read metaData: %#v", m)

	}

}
