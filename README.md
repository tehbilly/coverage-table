## Go Test Coverage Table

This is a small utility I wrote to give me better insight into test coverage in go projects. It will run `go test` for
you, saving coverage output to a temporary file, and then generate a table. The benefits of this over the built-in tools
_(for me, at least)_ is that it tries to be smart about what files it includes, generates an overall coverage
percentage, and includes go files that don't have tests.

#### Installation

Considering this is only useful in the context of examining go test coverage, installation is geared towards simply
using `go get github.com/tehbilly/coverage-table`. 

#### Usage

Run `coverage-table` in a directory containing a `go.mod` file, or pass a directory containing a `go.mod` file as the
first argument.
