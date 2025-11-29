package foo

import (
	"fmt"

	"github.com/pkg/errors"
)

func hi(some error) error {
	return errors.Wrapf(some, fmt.Sprintf("Hello %s", 1234))
}
