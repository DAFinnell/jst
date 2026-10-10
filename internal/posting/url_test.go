package posting

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			"case tracking fragment and query ordering",
			" HTTPS://Example.COM/jobs/123?utm_source=email&b=2&a=1#apply ",
			"https://example.com/jobs/123?a=1&b=2",
		},
		{
			"empty root path",
			"https://example.com",
			"https://example.com/",
		},
		{
			"tracking parameter names ignore case",
			"https://example.com/jobs/123?UTM_Source=email&GCLID=x&fbclid=y&job_id=123",
			"https://example.com/jobs/123?job_id=123",
		},
		{
			"job parameters path case and trailing slash",
			"https://example.com/Jobs/123/?source=board&job_id=123&ref=abc",
			"https://example.com/Jobs/123/?job_id=123&ref=abc&source=board",
		},
		{
			"repeated values retain their order",
			"https://example.com/jobs?b=2&a=first&a=second",
			"https://example.com/jobs?a=first&a=second&b=2",
		},
		{
			"escaped path separator",
			"https://example.com/jobs/a%2Fb",
			"https://example.com/jobs/a%2Fb",
		},
		{
			"empty query",
			"https://example.com/jobs/123?",
			"https://example.com/jobs/123",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeURL(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("normalized URL = %q, want %q", got, test.want)
			}

			again, err := NormalizeURL(got)
			if err != nil {
				t.Fatal(err)
			}
			if again != got {
				t.Fatalf("normalizing twice changed %q to %q", got, again)
			}
		})
	}
}

func TestNormalizeURLKeepsDistinctURLsDistinct(t *testing.T) {
	cases := []struct {
		name   string
		first  string
		second string
	}{
		{"scheme",
			"http://example.com/jobs/123",
			"https://example.com/jobs/123"},
		{"path case",
			"https://example.com/Jobs/123",
			"https://example.com/jobs/123"},
		{"trailing slash",
			"https://example.com/jobs/123",
			"https://example.com/jobs/123/"},
		{"job ID",
			"https://example.com/jobs?job_id=123",
			"https://example.com/jobs?job_id=456"},
		{"query value case",
			"https://example.com/jobs?job_id=ABC",
			"https://example.com/jobs?job_id=abc"},
		{"query key case",
			"https://example.com/jobs?Job_ID=123",
			"https://example.com/jobs?job_id=123"},
		{"repeated value order",
			"https://example.com/jobs?id=123&id=456",
			"https://example.com/jobs?id=456&id=123"},
		{"escaped separator",
			"https://example.com/jobs/a%2Fb",
			"https://example.com/jobs/a/b"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			first, err := NormalizeURL(test.first)
			if err != nil {
				t.Fatal(err)
			}

			second, err := NormalizeURL(test.second)
			if err != nil {
				t.Fatal(err)
			}

			if first == second {
				t.Fatalf("distinct URLs normalized to the same value: %q", first)
			}
		})
	}
}

func TestNormalizeURLRejectsInvalidURLs(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace", " \t\n"},
		{"relative", "/jobs/123"},
		{"missing scheme", "example.com/jobs/123"},
		{"scheme-relative", "//example.com/jobs/123"},
		{"unsupported scheme", "ftp://example.com/jobs/123"},
		{"missing host", "https:///jobs/123"},
		{"opaque URL", "https:jobs/123"},
		{"username", "https://user@example.com/jobs/123"},
		{"password", "https://user:password@example.com/jobs/123"},
		{"internal whitespace", "https://example.com/jobs/123?q=hello world"},
		{"invalid path escape", "https://example.com/jobs/%zz"},
		{"invalid query escape", "https://example.com/jobs?job_id=%zz"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeURL(test.input)
			if err == nil {
				t.Fatalf("expected URL rejection, got %q", got)
			}
			if got != "" {
				t.Fatalf("invalid URL returned comparison value %q", got)
			}
		})
	}
}
