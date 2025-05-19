package mapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdjustExitName(t *testing.T) {
	cases := []struct {
		in        string
		wantName  string
		wantDir   string
		wantError bool
	}{
		// plain cardinal
		{"east", "east", "east", false},
		// special cardinal
		{"east-x2", "east", "east-x2", false},
		// freeform noun
		{"cave", "cave", "", false},
		// noun + compass
		{"cave:south", "cave", "south", false},
		// noun + special compass
		{"cave:south-x2", "cave", "south-x2", false},
		// invalid special direction
		{"foo-x0", "foo-x0", "", true},
		// invalid colon direction
		{"cave:unknown", "cave", "", false},
		// over-parameterized
		{"east-x2:east-x2", "east-x2:east-x2", "", true},
		{"east-x2:east", "east-x2:east", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			name, dir, err := AdjustExitName(tc.in)
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantDir, dir)
		})
	}
}
