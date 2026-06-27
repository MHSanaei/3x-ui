package panel

import "testing"

func TestApiTokenCreatedAtSeconds(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want int64
	}{
		{name: "seconds", in: 1_782_485_394, want: 1_782_485_394},
		{name: "legacy milliseconds", in: 1_782_485_394_270, want: 1_782_485_394},
		{name: "unset", in: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apiTokenCreatedAtSeconds(tt.in); got != tt.want {
				t.Fatalf("apiTokenCreatedAtSeconds(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
