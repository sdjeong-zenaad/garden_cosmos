package garden

import (
	"context"
	"strings"
	"testing"
)

func TestFilterText(t *testing.T) {
	for _, tt := range []struct {
		name, input, pattern, want string
		fail                       bool
	}{
		{"regex", "INFO ok\nERROR fail\nWARN retry\n", "ERROR|WARN", "ERROR fail\nWARN retry\n", false},
		{"none", "hello\n", "absent", "", false},
		{"all", "first\nsecond\n", "", "first\nsecond\n", false},
		{"dash", "--version\nhello\n", "--version", "--version\n", false},
		{"unicode", "정상\n오류 발생\n", "오류", "오류 발생\n", false},
		{"invalid", "hello", "[", "", true},
		{"shell syntax", "hello\n", "; echo injected", "", false},
		{"limit", strings.Repeat("x", OutputLimit+1), "", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := FilterText(context.Background(), tt.input, tt.pattern)
			if (err != nil) != tt.fail || out != tt.want {
				t.Fatalf("output length=%d error=%v", len(out), err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FilterText(ctx, "hello", "."); err == nil {
		t.Fatal("cancellation ignored")
	}
}
