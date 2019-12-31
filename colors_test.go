package yacspin

import "testing"

func Test_validColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  bool
	}{
		{
			name:  "invalid",
			color: "invalid",
			want:  false,
		},
		{
			name:  "valid",
			color: "fgHiGreen",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validColor(tt.color); got != tt.want {
				t.Fatalf("validColor(%q) = %t, want %t", tt.color, got, tt.want)
			}
		})
	}
}
