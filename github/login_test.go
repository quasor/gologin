package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"goji.io"
	"github.com/quasor/gologin"
	oauth2Login "github.com/quasor/gologin/oauth2"
	"github.com/quasor/gologin/testutils"
	"github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

func TestGithubHandler(t *testing.T) {
	jsonData := `{"id": 917408, "name": "Alyssa Hacker"}`
	expectedUser := &github.User{ID: github.Int(917408), Name: github.String("Alyssa Hacker")}
	proxyClient, server := newGithubTestServer(jsonData)
	defer server.Close()
	// oauth2 Client will use the proxy client's base Transport
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, proxyClient)
	anyToken := &oauth2.Token{AccessToken: "any-token"}
	ctx = oauth2Login.WithToken(ctx, anyToken)

	config := &oauth2.Config{}
	success := func(ctx context.Context, w http.ResponseWriter, req *http.Request) {
		githubUser, err := UserFromContext(ctx)
		assert.Nil(t, err)
		assert.Equal(t, expectedUser, githubUser)
		fmt.Fprintf(w, "success handler called")
	}
	failure := testutils.AssertFailureNotCalled(t)

	// GithubHandler assert that:
	// - Token is read from the ctx and passed to the Github API
	// - github User is obtained from the Github API
	// - success handler is called
	// - github User is added to the ctx of the success handler
	githubHandler := githubHandler(config, goji.HandlerFunc(success), failure)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	githubHandler.ServeHTTP(ctx, w, req)
	assert.Equal(t, "success handler called", w.Body.String())
}

func TestGithubHandler_MissingCtxToken(t *testing.T) {
	config := &oauth2.Config{}
	success := testutils.AssertSuccessNotCalled(t)
	failure := func(ctx context.Context, w http.ResponseWriter, req *http.Request) {
		err := gologin.ErrorFromContext(ctx)
		if assert.NotNil(t, err) {
			assert.Equal(t, "oauth2: Context missing Token", err.Error())
		}
		fmt.Fprintf(w, "failure handler called")
	}

	// GithubHandler called without Token in ctx, assert that:
	// - failure handler is called
	// - error about ctx missing token is added to the failure handler ctx
	githubHandler := githubHandler(config, success, goji.HandlerFunc(failure))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	githubHandler.ServeHTTP(context.Background(), w, req)
	assert.Equal(t, "failure handler called", w.Body.String())
}

func TestGithubHandler_ErrorGettingUser(t *testing.T) {
	proxyClient, server := testutils.NewErrorServer("Github Service Down", http.StatusInternalServerError)
	defer server.Close()
	// oauth2 Client will use the proxy client's base Transport
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, proxyClient)
	anyToken := &oauth2.Token{AccessToken: "any-token"}
	ctx = oauth2Login.WithToken(ctx, anyToken)

	config := &oauth2.Config{}
	success := testutils.AssertSuccessNotCalled(t)
	failure := func(ctx context.Context, w http.ResponseWriter, req *http.Request) {
		err := gologin.ErrorFromContext(ctx)
		if assert.NotNil(t, err) {
			assert.Equal(t, ErrUnableToGetGithubUser, err)
		}
		fmt.Fprintf(w, "failure handler called")
	}

	// GithubHandler cannot get Github User, assert that:
	// - failure handler is called
	// - error cannot get Github User added to the failure handler ctx
	githubHandler := githubHandler(config, success, goji.HandlerFunc(failure))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	githubHandler.ServeHTTP(ctx, w, req)
	assert.Equal(t, "failure handler called", w.Body.String())
}

func TestValidateResponse(t *testing.T) {
	validUser := &github.User{ID: github.Int(123)}
	validResponse := &github.Response{Response: &http.Response{StatusCode: 200}}
	invalidResponse := &github.Response{Response: &http.Response{StatusCode: 500}}
	assert.Equal(t, nil, validateResponse(validUser, validResponse, nil))
	assert.Equal(t, ErrUnableToGetGithubUser, validateResponse(validUser, validResponse, fmt.Errorf("Server error")))
	assert.Equal(t, ErrUnableToGetGithubUser, validateResponse(validUser, invalidResponse, nil))
	assert.Equal(t, ErrUnableToGetGithubUser, validateResponse(&github.User{}, validResponse, nil))
}
