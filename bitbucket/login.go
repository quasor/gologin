package bitbucket

import (
	"errors"
	"net/http"

	"goji.io"
	"github.com/quasor/gologin"
	oauth2Login "github.com/quasor/gologin/oauth2"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

// Bitbucket login errors
var (
	ErrUnableToGetBitbucketUser = errors.New("bitbucket: unable to get Bitbucket User")
)

// StateHandler checks for a state cookie. If found, the state value is read
// and added to the ctx. Otherwise, a non-guessable value is added to the ctx
// and to a (short-lived) state cookie issued to the requester.
//
// Implements OAuth 2 RFC 6749 10.12 CSRF Protection. If you wish to issue
// state params differently, write a ContextHandler which sets the ctx state,
// using oauth2 WithState(ctx, state) since it is required by LoginHandler
// and CallbackHandler.
func StateHandler(config gologin.CookieConfig, success goji.Handler) goji.Handler {
	return oauth2Login.StateHandler(config, success)
}

// LoginHandler handles Bitbucket login requests by reading the state value
// from the ctx and redirecting requests to the AuthURL with that state value.
func LoginHandler(config *oauth2.Config, failure goji.Handler) goji.Handler {
	return oauth2Login.LoginHandler(config, failure)
}

// CallbackHandler handles Bitbucket redirection URI requests and adds the
// Bitbucket access token and User to the ctx. If authentication succeeds,
// handling delegates to the success handler, otherwise to the failure
// handler.
func CallbackHandler(config *oauth2.Config, success, failure goji.Handler) goji.Handler {
	success = bitbucketHandler(config, success, failure)
	return oauth2Login.CallbackHandler(config, success, failure)
}

// bitbucketHandler is a ContextHandler that gets the OAuth2 Token from the ctx
// to get the corresponding Bitbucket User. If successful, the User is added to
// the ctx and the success handler is called. Otherwise, the failure handler is
// called.
func bitbucketHandler(config *oauth2.Config, success, failure goji.Handler) goji.Handler {
	if failure == nil {
		failure = gologin.DefaultFailureHandler
	}
	fn := func(ctx context.Context, w http.ResponseWriter, req *http.Request) {
		token, err := oauth2Login.TokenFromContext(ctx)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(ctx, w, req)
			return
		}
		httpClient := config.Client(ctx, token)
		bitbucketClient := newClient(httpClient)
		user, resp, err := bitbucketClient.CurrentUser()
		err = validateResponse(user, resp, err)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(ctx, w, req)
			return
		}
		ctx = WithUser(ctx, user)
		success.ServeHTTP(ctx, w, req)
	}
	return goji.HandlerFunc(fn)
}

// validateResponse returns an error if the given Bitbucket User, raw
// http.Response, or error are unexpected. Returns nil if they are valid.
func validateResponse(user *User, resp *http.Response, err error) error {
	if err != nil || resp.StatusCode != http.StatusOK {
		return ErrUnableToGetBitbucketUser
	}
	if user == nil || user.Username == "" {
		return ErrUnableToGetBitbucketUser
	}
	return nil
}
