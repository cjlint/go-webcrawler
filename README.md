# go-webcrawler
Web crawler written in Go as an example project. Parallelizes HTTP calls
with goroutines.
To simplify things, it only follows absolute https URLs
found in the href attribute of `<a>` tags.
Crawls up to a specified depth (defaults to 3) or crawls
indefinitely if the user sets `-depth 0`.
I did not implement a limit to the amount of concurrency in the program,
so crawling indefinitely could end with the program running out of
resources.

## How to Run

Ensure go is installed:
```
brew install go
```
Or follow the [official installation instructions](https://go.dev/doc/install)

### Install Binary

```
go install github.com/cjlint/go-webcrawler@latest
go-webcrawler -url google.com
```

### From the Repository

Clone the repository, then run from the repository root:

```
go build
./go-webcrawler -url google.com
```

### Pipe to Output File

Go logs write to stderr by default, so use `2>&1` if you want to collect the logs in
an output file

```
go-webcrawler -url google.com -depth 3 > out 2>&1
```

## Tests

A small unit test suite is included in `main_test.go`

```
go test
```

Integration tests were not included due to time constraints, but they could be written
with the [httptest](https://pkg.go.dev/net/http/httptest) package. The source code
may need to be modified so that there is testable output from the program other
than asynchronous logs.

## Examples Used
- https://pkg.go.dev/golang.org/x/net/html#example-Parse for HTML parsing code
