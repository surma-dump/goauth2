// Copyright 2011 The goauth2 Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

var requests = []struct {
	path, query, auth string // request
	body              string // response
}{
	{
		path:  "/token",
		query: "grant_type=authorization_code&code=c0d3&redirect_uri=oob&client_id=cl13nt1d",
		body: `
			{
				"access_token":"token1",
				"refresh_token":"refreshtoken1",
				"expires_in":3600
			}
		`,
	},
	{path: "/secure", auth: "Bearer token1", body: "first payload"},
	{
		path:  "/token",
		query: "grant_type=refresh_token&refresh_token=refreshtoken1&client_id=cl13nt1d",
		body: `
			{
				"access_token":"token2",
				"refresh_token":"refreshtoken2",
				"expires_in":3600
			}
		`,
	},
	{path: "/secure", auth: "Bearer token2", body: "second payload"},
}

func TestOAuth(t *testing.T) {
	// Set up test server.
	n := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if n >= len(requests) {
			t.Errorf("too many requests: %d", n)
			return
		}
		req := requests[n]
		n++

		// Check request.
		if g, w := r.URL.Path, req.path; g != w {
			t.Errorf("request[%d] got path %s, want %s", n, g, w)
		}
		want, _ := url.ParseQuery(req.query)
		for k := range want {
			if g, w := r.FormValue(k), want.Get(k); g != w {
				t.Errorf("query[%s] = %s, want %s", k, g, w)
			}
		}
		if g, w := r.Header.Get("Authorization"), req.auth; w != "" && g != w {
			t.Errorf("Authorization: %v, want %v", g, w)
		}

		// Send response.
		io.WriteString(w, req.body)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	config := &Config{
		ClientId:     "cl13nt1d",
		ClientSecret: "s3cr3t",
		Scope:        "https://example.net/scope",
		AuthURL:      server.URL + "/auth",
		TokenURL:     server.URL + "/token",
	}

	// TODO(adg): test AuthCodeURL

	transport := &Transport{Config: config}
	_, err := transport.Exchange("c0d3")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	checkToken(t, transport.Token, "token1", "refreshtoken1")

	c := transport.Client()
	resp, err := c.Get(server.URL + "/secure")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	checkBody(t, resp, "first payload")

	// test automatic refresh
	transport.Expiry = time.Now()
	resp, err = c.Get(server.URL + "/secure")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	checkBody(t, resp, "second payload")
	checkToken(t, transport.Token, "token2", "refreshtoken2")
}

func checkToken(t *testing.T, tok *Token, access, refresh string) {
	if g, w := tok.AccessToken, access; g != w {
		t.Errorf("AccessToken = %q, want %q", g, w)
	}
	if g, w := tok.RefreshToken, refresh; g != w {
		t.Errorf("RefreshToken = %q, want %q", g, w)
	}
	exp := tok.Expiry.Sub(time.Now())
	if (time.Hour-time.Second) > exp || exp > time.Hour {
		t.Errorf("Expiry = %v, want ~1 hour", exp)
	}
}

func checkBody(t *testing.T, r *http.Response, body string) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Error("reading reponse body: %v, want %q", err, body)
	}
	if g, w := string(b), body; g != w {
		t.Errorf("request body mismatch: got %q, want %q", g, w)
	}
}
