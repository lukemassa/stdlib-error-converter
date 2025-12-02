# unpkg-errors

Simple program to convert code that uses the deprecated [pkg/errors](https://github.com/pkg/errors) into calls to the standard library [errors](https://pkg.go.dev/errors).

## Installation

```
go install github.com/lukemassa/unpkg-errors/cmd/unpkg-errors@latest
```

## Basic Usage

Point it at a file and, similar in spirit to goimports, it will replace the content with the corrected one

```
unpkg-errors % unpkg-errors internal/processor/testdata/wrapf.go 
I 2025/12/01 23:46:31.946 internal/processor/testdata/wrapf.go: Fixed 1 references to pkg/errors
package foo

import (
	"fmt"
)

func hi(some error) error {
	return fmt.Errorf("Hello %s: %w", 1234, some)
}
```

## Known bugs/limitations

When the code is unable to do a conversion, it prints a warning but continues to convert other aspects of the file.

- For Wrap(), we should check that err != nil first (since fmt.Errorf() and errors.Wrap treat nil differently)
- Cannot convert Unwrap()
- Cannot Cause() (maybe we can use Is somehow?)
- Doesn't handle pkg/errors being imported under a different name (or errors for that matter)
