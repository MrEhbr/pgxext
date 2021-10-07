package cluster

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/hashicorp/go-multierror"
)

func TestScatter(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	t.Run("have errors", func(t *testing.T) {
		seq := []int{1, 2, 3, 4, 5, 6, 7, 8}
		err := scatter(len(seq), func(i int) error {
			if seq[i]%2 == 0 {
				seq[i] *= seq[i]
				return nil
			}
			return fmt.Errorf("%d is an odd fellow", seq[i])
		})

		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		errs, ok := err.(*multierror.Error)
		if !ok {
			t.Fatalf("Expect multierror, got %+v", err)
		}

		if errs.Len() != 4 {
			t.Fatalf("Excpect 4 errors, got: %d", errs.Len())
		}

		want := []int{1, 4, 3, 16, 5, 36, 7, 64}
		for i := range want {
			if want[i] != seq[i] {
				t.Errorf("Wrong value at position %d. Want: %d, Got: %d", i, want[i], seq[i])
			}
		}
	})
	t.Run("no errors", func(t *testing.T) {
		seq := []int{2, 4, 6, 8}
		err := scatter(len(seq), func(i int) error {
			if seq[i]%2 == 0 {
				seq[i] *= seq[i]
				return nil
			}
			return fmt.Errorf("%d is an odd fellow", seq[i])
		})
		if err != nil {
			t.Fatalf("Expected nil, got error: %s", err)
		}

		want := []int{4, 16, 36, 64}
		for i := range want {
			if want[i] != seq[i] {
				t.Errorf("Wrong value at position %d. Want: %d, Got: %d", i, want[i], seq[i])
			}
		}
	})
}
