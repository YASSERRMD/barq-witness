package main

import "testing"

func TestScanForSecrets_AWSKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid AWS key",
			input: "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			want:  true,
		},
		{
			name:  "AWS key in diff line",
			input: "+aws_access_key_id = AKIAIOSFODNN7EXAMPLEX",
			want:  true,
		},
		{
			name:  "no AWS key",
			input: "this is just a normal line of code",
			want:  false,
		},
		{
			name:  "AKIA prefix but too short",
			input: "AKIATOOSHORT",
			want:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScanForSecrets(tc.input)
			if got != tc.want {
				t.Errorf("ScanForSecrets(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestScanForSecrets_GenericSecret(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "password assignment double quotes",
			input: `password = "supersecretpassword123"`,
			want:  true,
		},
		{
			name:  "api_key assignment single quotes",
			input: `api_key = 'my-api-key-value-here'`,
			want:  true,
		},
		{
			name:  "token assignment",
			input: `token="ghp_longerthan8chars"`,
			want:  true,
		},
		{
			name:  "secret keyword",
			input: `SECRET="someSecretValue123"`,
			want:  true,
		},
		{
			name:  "short value under threshold",
			input: `password = "short"`,
			want:  false,
		},
		{
			name:  "unrelated line",
			input: "func computeHash(data []byte) string {",
			want:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScanForSecrets(tc.input)
			if got != tc.want {
				t.Errorf("ScanForSecrets(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestScanForSecrets_EmptyInput(t *testing.T) {
	if ScanForSecrets("") {
		t.Error("expected false for empty input")
	}
}
