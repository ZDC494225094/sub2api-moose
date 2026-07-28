package service

import "testing"

func TestNormalizeAPIKeyPlatformIncludesEveryConcreteGroupPlatform(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: PlatformAnthropic, want: PlatformAnthropic},
		{input: PlatformOpenAI, want: PlatformOpenAI},
		{input: PlatformGemini, want: PlatformGemini},
		{input: PlatformAntigravity, want: PlatformAntigravity},
		{input: PlatformGrok, want: PlatformGrok},
		{input: " GROK ", want: PlatformGrok},
		{input: PlatformComposite, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeAPIKeyPlatform(tt.input); got != tt.want {
				t.Fatalf("NormalizeAPIKeyPlatform(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
