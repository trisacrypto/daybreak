package auth

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/trisacrypto/daybreak/pkg"
	"go.rtnl.ai/x/gravatar"
	"golang.org/x/oauth2"
)

func init() {
	gob.Register(UserInfo{})
}

const userinfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

type UserInfo struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	HD            string `json:"hd"`
	FamilyName    string `json:"family_name"`
	GivenName     string `json:"given_name"`
	Picture       string `json:"picture"`
	Gravatar      string `json:"-"`
}

func Profile(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	var (
		err error
		req *http.Request
		rep *http.Response
	)

	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, userinfoURL, nil); err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("daybreak/%s", pkg.Version(false)))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	if rep, err = http.DefaultClient.Do(req); err != nil {
		return nil, err
	}
	defer rep.Body.Close()

	userinfo := &UserInfo{}
	if err = json.NewDecoder(rep.Body).Decode(&userinfo); err != nil {
		return nil, err
	}

	userinfo.Gravatar = gravatar.New(userinfo.Email, nil)
	return userinfo, nil
}
