package browserlogin

import "testing"

func TestCodeFromURL(t *testing.T) {
	cases := []struct {
		name, store, raw, want string
	}{
		{"epic localhost", "epic", "http://localhost/?code=epic-code", "epic-code"},
		{"epic redirect json", "epic", "https://www.epicgames.com/id/api/redirect?authorizationCode=epic-json", "epic-json"},
		{"epic login page is not a result", "epic", "https://www.epicgames.com/id/login?responseType=code", ""},
		{"gog success page", "gog", "https://embed.gog.com/on_login_success?code=gog-code&origin=client", "gog-code"},
		{"gog login page is not a result", "gog", "https://auth.gog.com/auth?client_id=x&response_type=code", ""},
		{"amazon authorization code", "amazon", "https://www.amazon.com/ap/oa?openid.oa2.authorization_code=amazon-code&x=1", "amazon-code"},
		{"amazon login page is not a result", "amazon", "https://amazon.com/ap/signin?openid.oa2.response_type=code", ""},
		{"unknown store", "steam", "https://example.test/?code=x", ""},
		{"garbage", "epic", "://not a url", ""},
	}
	for _, c := range cases {
		if got := CodeFromURL(c.store, c.raw); got != c.want {
			t.Errorf("%s: CodeFromURL(%q) = %q, want %q", c.name, c.raw, got, c.want)
		}
	}
}

func TestLoginURL(t *testing.T) {
	if got := LoginURL("epic", "https://legendary.gl/epiclogin"); got != EpicLoginURL {
		t.Errorf("epic auto URL = %q, want %q", got, EpicLoginURL)
	}
	if got := LoginURL("gog", "https://auth.gog.com/auth"); got != "https://auth.gog.com/auth" {
		t.Errorf("gog auto URL lost the provider URL: %q", got)
	}
}
