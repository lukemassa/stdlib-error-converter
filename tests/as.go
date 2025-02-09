package foo

import "github.com/pkg/errors"

func hi() error {
	return errors.As("foo", nil)
}
