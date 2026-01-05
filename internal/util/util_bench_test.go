package util

import (
	"testing"
)

func BenchmarkExtractDomain(b *testing.B) {
	urls := []string{
		"https://example.com/webhook",
		"https://api.example.com/v1/hooks",
		"https://subdomain.api.example.com:8443/path",
		"http://localhost:8080/test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractDomain(urls[i%len(urls)])
	}
}

func BenchmarkCalculateBackoff(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateBackoff(i%10+1, 1000, 250)
	}
}

func BenchmarkIsRetryableHTTPStatus(b *testing.B) {
	statuses := []int{200, 400, 429, 500, 502, 503}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsRetryableHTTPStatus(statuses[i%len(statuses)])
	}
}
