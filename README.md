gotar is a drop-in replacement for `go build` that also includes any static
files (e.g. html, templates, and javascript) within the resulting binary.

Installation
------------

    go install github.com/ConradIrwin/gotar

Usage
-----

Instead of running `go build`, run `gotar`. That's it!

You can continue to use `$GOOS` and `$GOARCH` to control cross-compilation as normal.

No changes to your app code are required unless you rely on the working directory. See [Notes](#Notes).

Notes
-----

gotar works by creating a self-extracting tar file with your app in it. When the gotar app is run it:

1. exports the current working directory to the `$GOTAR_PWD` environment variable
1. creates a temporary directory (and exports as `$GOTAR_DIR`)
1. cds into `$GOTAR_DIR`
1. untars all of your files
1. execs your app.

If your app doesn't depend on its working directory (e.g. servers) then this
will work flawlessly. However if you need access to the working directory you
now need to access it via `os.Getenv("GOTAR_PWD")` instead of `os.Getwd()`.

The current implementation creates a new temporary directory each time the
application is run. You can run `rm -r $GOTAR_DIR` when your app quits to
reclaim this space.

Configuration
-------------

By default gotar will include the entire current directory and all subdirectories.
If you only want a subset of your files to be included, list them in a `.gotar` file
at the top level. This file consists of several patterns, one per line. If the pattern
contains a `**` then it will match all files in the specified directories and subdirectories.Otherwise the pattern is passed to [`filepath.Match`](https://godoc.org/path/filepath/#Match).

Meta-fu
-------

gotar is licensed under the MIT license (see LICENSE.MIT). Contributions and bug reports are welcome.
