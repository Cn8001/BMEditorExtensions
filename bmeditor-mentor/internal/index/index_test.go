package index

import "testing"

func TestSplit(t *testing.T) {
	cases := map[string]string{
		"UserIdentityID": "user identity id",
		"HTTPServer":     "http server",
		"provider_name":  "provider name",
		"newSession":     "new session",
		"OAuth2Token":    "o auth2 token",
	}
	for in, want := range cases {
		got := ""
		for i, w := range Split(in) {
			if i > 0 {
				got += " "
			}
			got += w
		}
		if got != want {
			t.Errorf("Split(%q) = %q, want %q", in, got, want)
		}
	}
}
