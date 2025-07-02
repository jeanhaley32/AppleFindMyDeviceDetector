package util

import (
	"testing"
)

func TestMin(t *testing.T) {
	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{
			name: "a is smaller",
			a:    1,
			b:    2,
			want: 1,
		},
		{
			name: "b is smaller",
			a:    2,
			b:    1,
			want: 1,
		},
		{
			name: "a and b are equal",
			a:    1,
			b:    1,
			want: 1,
		},
		{
			name: "negative numbers",
			a:    -5,
			b:    -10,
			want: -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Min(tt.a, tt.b); got != tt.want {
				t.Errorf("Min(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
