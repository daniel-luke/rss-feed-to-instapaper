package instapaper

import (
	"net/url"
	"testing"
)

// Test vector from OAuth Core 1.0 spec §A.5.
// Expected base string and signature are specified in the RFC.
func TestSign_known_vector(t *testing.T) {
	bodyParams := url.Values{
		"file": {"vacation.jpg"},
		"size": {"original"},
	}
	got := sign(
		"GET",
		"http://photos.example.net/photos",
		bodyParams,
		"dpf43f3p2l4k3l03", // consumerKey
		"kd94hf93k423kf44", // consumerSecret
		"nnch734d00sl2jdk", // token
		"pfkkdhi9sl3r4s00", // tokenSecret
		"kllo9940pd9333jh", // nonce
		"1191242096",       // timestamp
	)
	want := "tR3+Ty81lMeYAr/Fid0kMTYa/WM="
	if got != want {
		t.Errorf("sign: got %q, want %q", got, want)
	}
}

func TestSign_deterministic(t *testing.T) {
	params := url.Values{"url": {"https://example.com/article"}}
	a := sign("POST", "https://www.instapaper.com/api/1.1/bookmarks/add",
		params, "key", "secret", "tok", "toksecret", "nonce123", "1700000000")
	b := sign("POST", "https://www.instapaper.com/api/1.1/bookmarks/add",
		params, "key", "secret", "tok", "toksecret", "nonce123", "1700000000")
	if a != b {
		t.Errorf("sign not deterministic: %q != %q", a, b)
	}
}

func TestSign_different_secrets_differ(t *testing.T) {
	params := url.Values{"url": {"https://example.com"}}
	a := sign("POST", "https://example.com/api", params, "key", "secretA", "", "", "n", "1")
	b := sign("POST", "https://example.com/api", params, "key", "secretB", "", "", "n", "1")
	if a == b {
		t.Error("different consumer secrets must produce different signatures")
	}
}
