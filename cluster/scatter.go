package cluster

import "github.com/hashicorp/go-multierror"

func scatter(n int, fn func(i int) error) error {
	errors := make(chan error, n)

	for i := range n {
		go func(i int) { errors <- fn(i) }(i)
	}

	var (
		errs     *multierror.Error
		innerErr error
	)
	for range cap(errors) {
		if innerErr = <-errors; innerErr != nil {
			errs = multierror.Append(errs, innerErr)
		}
	}

	return errs.ErrorOrNil()
}
