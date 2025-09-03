package foo

import "github.com/pkg/errors"

func hi(some error) error {
	return errors.Wrap("foo")
}
