// This package implements a decoder for gotar apps.
// It is responsible for untarring the file and execing the actual app.
package main

import (
	"github.com/ConradIrwin/gotar/format"
	"github.com/mitchellh/osext"
	"io"
	"io/ioutil"
	"os"
	"path"
	"syscall"
)

type metaData struct {
	Name string
}

func main() {
	pwd, err := os.Getwd()
    if err != nil {
        panic(err)
    }

    pth, err := osext.Executable()

    if err != nil {
		panic(err)
    }

    file, err := os.Open(pth)

    if err != nil {
		panic(err)
    }

	archive, err := gotar.Read(file)
	if err != nil {
		panic(err)
	}

	dir, err := ioutil.TempDir("", "")
    if err != nil {
        panic(err)
    }
	os.Setenv("GOTAR_PWD", pwd)
	os.Setenv("GOTAR_DIR", dir)

	var m metaData
	err = archive.ReadMetaData(&m)
	if err != nil {
		panic(err)
	}

	os.Chdir(dir)

	err = Unpack(archive)
	if err != nil {
		panic(err)
	}

	err = syscall.Exec(path.Join(dir, m.Name), os.Args, os.Environ())
	if err != nil {
		panic(err)
	}
}

func Unpack(archive *gotar.Archive) error {

    for {
        hdr, contents, err := archive.NextFile()
        if err != nil {
            if err == io.EOF || err == io.ErrUnexpectedEOF {
                return nil
            }
			panic(err)
        }

		err = os.MkdirAll(path.Dir(hdr.Name), 0755)
		if err != nil {
			panic(err)
		}

        file, err := os.Create(hdr.Name)
        if err != nil {
			panic(err)
        }

        if hdr.Size > 0 {
            _, err := io.Copy(file, contents)
			if err != nil {
				panic(err)
			}
        }

		err = file.Close()
		if err != nil {
			panic(err)
		}
		err = os.Chmod(file.Name(), os.FileMode(hdr.Mode))
		if err != nil {
			panic(err)
		}
    }

    return nil
}
