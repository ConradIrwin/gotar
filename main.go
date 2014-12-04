// gotar is a tool for building a go app with all of its static dependencies
// included.
package main

import (
	"fmt"
	"github.com/ConradIrwin/gotar/format"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

type configFile struct {
	patterns []string
}

func main() {

	if len(os.Args) > 1 {

		fmt.Println(`gotar [--help]

gotar builds your go app, including any static files in the current directory (e.g.
templates, html files, config files).

When your app executes, a temporary directory is created with all of your static
content and your app is run from within that directory.

Two environment variables are exported:
* GOTAR_DIR the new working directory that contains all your static files
* GOTAR_PWD the working directory when the app was invoked (if your app currently
uses os.Getwd(), you can use os.Getenv("GOTAR_PWD") instead).

You can cross-compile apps with gotar just as you would with go build, export
the GOOS and GOARCH environment variables.

If a .gotar is present in the directory in which gotar is run, then each line of
that file is treated as a pattern. Only files which match the patterns will be
included.
`)

		return
	}

	GoBuild()

	decoder := GoBuildDecoder()

	t, err := ioutil.TempFile("", "gotar")
	if err != nil {
		panic(err)
	}

	archive := gotar.NewWriter(t)
	err = archive.WriteDecoder(decoder)
	if err != nil {
		panic(err)
	}
	addFiles(".", archive)

	archive.WriteMetaData(struct{ Name string }{AppName()})
	err = archive.Close()
	if err != nil {
		panic(err)
	}
	err = t.Close()
	if err != nil {
		panic(err)
	}

	err = os.Chmod(t.Name(), 0755)
	if err != nil {
		panic(err)
	}

	err = os.Rename(t.Name(), AppName())
	if err != nil {
		panic(err)
	}

	_ = os.Remove(decoder.Name())

}

func AppName() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return path.Base(dir)
}

func GoBuildDecoder() *os.File {
	decoder, err := ioutil.TempFile("", "decoder")
	if err != nil {
		panic(err)
	}
	err = decoder.Close()
	if err != nil {
		panic(err)
	}

	GoBuild("-o", decoder.Name(), "github.com/ConradIrwin/gotar/decoder")

	decoder, err = os.Open(decoder.Name())
	if err != nil {
		panic(err)
	}

	return decoder
}

func GoBuildApp() {
	GoBuild("-o", AppName())
}

func GoBuild(args ...string) {

	cmd := exec.Command("go", append([]string{"build"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		switch err := err.(type) {
		case *exec.ExitError:
			os.Exit(err.Sys().(syscall.WaitStatus).ExitStatus())
		default:
			panic(err)
		}
	}
}

func addFiles(dir string, archive *gotar.Writer) {

	config := configFile{}
	lines, err := ioutil.ReadFile(".gotar")
	if err == nil {
		for _, line := range strings.Split(string(lines), "\n") {
			config.patterns = append(config.patterns, line)
		}
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && config.match(path) {

			f, err := os.Open(path)
			if err != nil {
				return err
			}

			archive.WriteFile(f, dir)
		}

		return nil
	})

	if err != nil && err != io.EOF {
		panic(err)
	}
}

func (config *configFile) match(path string) bool {
	if len(config.patterns) == 0 {
		return true
	}

	if path == AppName() {
		return true
	}

	for i, pattern := range config.patterns {
		if strings.Contains(pattern, "**") {
			prefix := strings.Split(pattern, "**")[0]

			if strings.HasPrefix(path, prefix) {
				return true
			}
		} else {
			match, err := filepath.Match(pattern, path)
			if err != nil {
				fmt.Printf("Invalid pattern in .gotar on line %n: %s", i, pattern)
				os.Exit(1)
			}
			if match {
				return true
			}
		}
	}

	return false
}
