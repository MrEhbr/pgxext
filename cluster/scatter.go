package cluster

import "github.com/hashicorp/go-multierror"

func scatter(n int, fn func(i int) error) error {
	errors := make(chan error, n)

	var i int
	for i = 0; i < n; i++ {
		go func(i int) { errors <- fn(i) }(i)
	}

	var (
		errs     *multierror.Error
		innerErr error
	)
	for i = 0; i < cap(errors); i++ {
		if innerErr = <-errors; innerErr != nil {
			errs = multierror.Append(errs, innerErr)
		}
	}

	return errs.ErrorOrNil()
}
