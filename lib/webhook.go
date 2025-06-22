package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/m-tsuru/tenchi-geolocation/structs"
	"gorm.io/gorm"
)

type Database struct {
	*gorm.DB
}

type DiscordWebhookContent struct {
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Content   string `json:"content"`
}

func NotifyGeolocationUpdate(userDetail *structs.UserDetail, webhookURL *string, location *structs.Geolocation) error {
	if webhookURL == nil || *webhookURL == "" {
		return fmt.Errorf("webhookURL is empty")
	}
	if !strings.HasPrefix(*webhookURL, "http://") && !strings.HasPrefix(*webhookURL, "https://") {
		return fmt.Errorf("webhookURL must start with http:// or https://")
	}

	data := DiscordWebhookContent{
		Username:  "市内鬼ごっこ",
		AvatarURL: "https://lh3.googleusercontent.com/a/ACg8ocIAjvLfg1jNaC3znxz_Zy1AV6fCJ0aNUath8zBQwvzPhmZzUq0=s96-c",
		Content: fmt.Sprintf("`%s` の位置情報が ユーザ `%s` によって更新されました。\n位置情報: 緯度 %f, 経度 %f",
			userDetail.Team.Name,
			userDetail.UserProfile.UserName,
			location.Latitude,
			location.Longitude,
		),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	resp, err := http.Post(
		*webhookURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
