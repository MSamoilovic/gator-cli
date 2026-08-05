package feeds

import (
	"errors"
	"testing"

	"github.com/lib/pq"
)

func TestIsDuplicate(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "unique violation",
			err:  &pq.Error{Code: pqUniqueViolation},
			want: true,
		},
		{
			name: "wrapped unique violation",
			err:  errors.Join(errors.New("saving post"), &pq.Error{Code: pqUniqueViolation}),
			want: true,
		},
		{
			name: "other postgres error",
			err:  &pq.Error{Code: "23503"},
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("connection refused"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDuplicate(tc.err); got != tc.want {
				t.Errorf("isDuplicate(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
