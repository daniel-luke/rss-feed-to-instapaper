package instapaper

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"sort"
	"strings"
)

// sign computes an OAuth 1.0a HMAC-SHA1 signature.
// bodyParams holds request-specific params only (url, title, x_auth_*, etc.).
// OAuth params are assembled here from the explicit arguments.
// oauth_token is omitted when token is empty (xAuth initial exchange).
func sign(method, rawURL string, bodyParams url.Values, consumerKey, consumerSecret, token, tokenSecret, nonce, timestamp string) string {
	all := url.Values{}
	for k, vs := range bodyParams {
		all[k] = vs
	}
	all.Set("oauth_consumer_key", consumerKey)
	all.Set("oauth_nonce", nonce)
	all.Set("oauth_signature_method", "HMAC-SHA1")
	all.Set("oauth_timestamp", timestamp)
	all.Set("oauth_version", "1.0")
	if token != "" {
		all.Set("oauth_token", token)
	}

	var pairs []string
	for k, vs := range all {
		for _, v := range vs {
			pairs = append(pairs, pctEnc(k)+"="+pctEnc(v))
		}
	}
	sort.Strings(pairs)
	paramStr := strings.Join(pairs, "&")

	base := strings.ToUpper(method) + "&" + pctEnc(rawURL) + "&" + pctEnc(paramStr)
	key := pctEnc(consumerSecret) + "&" + pctEnc(tokenSecret)

	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(base))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// pctEnc percent-encodes per RFC 3986 §2.1 (spaces as %20, not +).
func pctEnc(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
