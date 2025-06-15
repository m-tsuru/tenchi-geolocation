package lib

import (
	"encoding/json"
	"time"

	"gopkg.in/ini.v1"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type ServerConfig struct {
	JWTTokenSecret string
}

func LoadConfig() (*oauth2.Config, *ServerConfig, error) {
	cfg, err := ini.Load(".env")
	if err != nil {
		return nil, nil, err
	}

	clientID := cfg.Section("Google").Key("ClientID").String()
	clientSecret := cfg.Section("Google").Key("ClientSecret").String()
	redirectURL := cfg.Section("Google").Key("RedirectURL").String()

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil, nil, fiber.NewError(fiber.StatusInternalServerError, "Google OAuth configuration is missing")
	}

	jwtTokenSecret := cfg.Section("Server").Key("JWTTokenSecret").String()
	if jwtTokenSecret == "" {
		return nil, nil, fiber.NewError(fiber.StatusInternalServerError, "JWT Token Secret is missing in configuration")
	}
	serverConfig := &ServerConfig{
		JWTTokenSecret: jwtTokenSecret,
	}
	if jwtTokenSecret == "" {
		return nil, nil, fiber.NewError(fiber.StatusInternalServerError, "JWT Token Secret is missing in configuration")
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}, serverConfig, nil
}

func GetGoogleOAuthURL(cfg *oauth2.Config) string {
	return cfg.AuthCodeURL("state", oauth2.AccessTypeOffline)
}

func GetTokenfromGoogle(c *fiber.Ctx, cfg *oauth2.Config) (*oauth2.Token, error) {
	code := c.Query("code")
	if code == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Missing authorization code")
	}

	token, err := cfg.Exchange(c.Context(), code)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to exchange token: "+err.Error())
	}

	return token, nil
}

func GetUserInfoFromGoogle(c *fiber.Ctx, cfg *oauth2.Config, token *oauth2.Token) (map[string]interface{}, error) {
	client := cfg.Client(c.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to get user info: "+err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		return nil, fiber.NewError(resp.StatusCode, "Failed to get user info: "+resp.Status)
	}

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to decode user info: "+err.Error())
	}

	return userInfo, nil
}

func LoginOperation(c *fiber.Ctx, cfg *oauth2.Config) (*map[string]interface{}, error) {
	token, err := GetTokenfromGoogle(c, cfg)
	if err != nil {
		// Handle error
		return nil, c.Status(fiber.StatusInternalServerError).SendString("Failed to get token: " + err.Error())
	}
	userInfo, err := GetUserInfoFromGoogle(c, cfg, token)
	if err != nil {
		// Handle error
		return nil, c.Status(fiber.StatusInternalServerError).SendString("Failed to get user info: " + err.Error())
	}
	return &userInfo, nil
}

func GenerateJWT(userID string, secret string) (*string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     jwt.TimeFunc().Add(24 * time.Hour).Unix(), // Token valid for 24 hours
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, err
	}
	return &signedToken, nil
}

func GetUserIDByJWT(tokenString string, secret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Invalid JWT token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Invalid JWT claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", fiber.NewError(fiber.StatusUnauthorized, "User ID not found in JWT claims")
	}

	return userID, nil
}

func CheckNotExpireJWT(tokenString string, secret string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return false
	}
	return int64(exp) > time.Now().Unix()
}
