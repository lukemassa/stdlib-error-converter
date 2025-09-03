package foo

import "github.com/pkg/errors"

func hi() error {
	return errors.Errorf("Hello")
}
