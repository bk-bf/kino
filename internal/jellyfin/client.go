// Package jellyfin is a wrapper around the `github.com/sj14/jellyfin-go/api` to make it easier to use
package jellyfin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sj14/jellyfin-go/api"
)

type Client struct {
	api    *api.APIClient
	Host   string
	UserID string
	Token  string
}

// get token and user id
func authorize(host, username, password, device, deviceID, version string) (token, userID string, err error) {
	authHeader := fmt.Sprintf("MediaBrowser Client=\"jfsh\", Device=%q, DeviceId=%q, Version=%q", device, deviceID, version)
	config := &api.Configuration{
		Servers:       api.ServerConfigurations{{URL: host}},
		DefaultHeader: map[string]string{"Authorization": authHeader},
	}
	cl := api.NewAPIClient(config)
	res, _, err := cl.UserAPI.AuthenticateUserByName(context.Background()).AuthenticateUserByName(api.AuthenticateUserByName{
		Username: *api.NewNullableString(&username),
		Pw:       *api.NewNullableString(&password),
	}).Execute()
	if err != nil {
		slog.Error("failed to authenticate", "err", err)
		return
	}
	token = *res.AccessToken.Get()
	userID = *res.GetUser().Id
	return
}

func NewClient(host, username, password, device, deviceID, version, token, userID string) (*Client, error) {
	if token == "" || userID == "" {
		newToken, newUserID, err := authorize(host, username, password, device, deviceID, version)
		if err != nil {
			return nil, err
		}
		token = newToken
		userID = newUserID
	}

	authHeader := fmt.Sprintf("MediaBrowser Client=\"jfsh\", Device=%q, DeviceId=%q, Version=%q, Token=%q", device, deviceID, version, token)
	config := &api.Configuration{
		Servers:       api.ServerConfigurations{{URL: host}},
		DefaultHeader: map[string]string{"Authorization": authHeader},
	}
	apiClient := api.NewAPIClient(config)
	return &Client{api: apiClient, Host: host, UserID: userID, Token: token}, nil
}
