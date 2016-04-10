package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	jwt "github.com/dgrijalva/jwt-go"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

const (
	OAuthURLGoogle = "https://www.googleapis.com/oauth2/v1/certs"
)

type OAuthKeys struct {
	mu   sync.RWMutex
	url  string
	keys map[string]interface{}
}

func NewOAuthKeys(ctx context.Context, url string) (*OAuthKeys, error) {
	o := &OAuthKeys{
		url:  url,
		keys: make(map[string]interface{}),
	}
	if err := o.Update(ctx); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *OAuthKeys) Update(ctx context.Context) error {
	resp, err := ctxhttp.Get(ctx, nil, o.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status != 200: %d", resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var s map[string]string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	nu := make(map[string]interface{})
	for k, v := range s {
		t, err := jwt.ParseRSAPublicKeyFromPEM([]byte(v))
		if err != nil {
			return err
		}
		nu[k] = t
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.keys = nu
	return nil
}

func (o *OAuthKeys) lookup(s string) (interface{}, error) {
	o.mu.RLock()
	v, found := o.keys[s]
	o.mu.RUnlock()
	if !found {
		return nil, fmt.Errorf("oauth key %q not found", s)
	}
	return v, nil
}

// verify JWT and return email and oauthSubject.
func (o *OAuthKeys) VerifyJWT(t string) (string, string, error) {
	parser := jwt.Parser{UseJSONNumber: true}
	token, err := parser.Parse(t, func(token *jwt.Token) (interface{}, error) {
		return o.lookup(token.Header["kid"].(string))
	})

	if err != nil {
		return "", "", err
	}
	if !token.Valid {
		return "", "", fmt.Errorf("invalid token")
	}
	if t, _ := token.Claims["email_verified"].(bool); !t {
		return "", "", fmt.Errorf("email not verified")
	}
	if t, _ := token.Claims["aud"].(string); t == "" || t != *oauthClientID {
		return "", "", fmt.Errorf("incorrect client ID %q", t)
	}
	if t, _ := token.Claims["iss"].(string); !validOAuthIssuers[t] {
		return "", "", fmt.Errorf("invalid issuer ID %q", t)
	}
	email, ok := token.Claims["email"].(string)
	if !ok || email == "" {
		return "", "", fmt.Errorf("missing email")
	}
	oauthSubject, ok := token.Claims["sub"].(string)
	if !ok || oauthSubject == "" {
		return "", "", fmt.Errorf("missing oauthSubject")
	}
	return email, oauthSubject, nil
}
