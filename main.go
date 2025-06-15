package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/m-tsuru/tenchi-geolocation/lib"
	"github.com/m-tsuru/tenchi-geolocation/structs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Requirelogin(db *gorm.DB, jwtTokenSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		jwtToken := c.Cookies("jwt")
		if jwtToken == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("JWT token is required")
		}
		if !lib.CheckNotExpireJWT(jwtToken, jwtTokenSecret) {
			return c.Status(fiber.StatusUnauthorized).SendString("JWT token is expired")
		}
		userID, err := lib.GetUserIDByJWT(jwtToken, jwtTokenSecret)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid JWT token: " + err.Error())
		}
		dbInstance := &structs.Database{DB: db}
		IsExist, err := dbInstance.GetUserDetailByID(userID)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user detail: " + err.Error())
		}
		if IsExist == nil {
			return c.Status(fiber.StatusUnauthorized).SendString("JWT Token is invalid")
		}
		return c.Next()
	}
}

func main() {
	oaCfg, svrCfg, err := lib.LoadConfig()
	if err != nil {
		// Handle error
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := gorm.Open(
		sqlite.Open("main.db"),
		&gorm.Config{},
	)

	dbInstance := &structs.Database{DB: db}
	err = dbInstance.AutoMigrateModels()
	if err != nil {
		// Handle error
		log.Fatalf("Failed to auto-migrate models: %v", err)
	}
	err = dbInstance.AutoCreateTestData()
	if err != nil {
		// Handle error
		log.Fatalf("Failed to auto-create test data: %v", err)
	}

	app := fiber.New()
	api := app.Group("/api")

	api.Get("/login", func(c *fiber.Ctx) error {
		authURL := lib.GetGoogleOAuthURL(oaCfg)
		return c.Redirect(authURL, fiber.StatusFound)
	})

	api.Get("/callback", func(c *fiber.Ctx) error {

		userCallback, err := lib.LoginOperation(c, oaCfg)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Login operation failed: " + err.Error())
		}

		dbInstance := &structs.Database{DB: db}
		id, ok := (*userCallback)["sub"]
		if !ok {
			return c.Status(fiber.StatusInternalServerError).SendString("User ID not found in callback")
		}
		idStr, ok := id.(string)
		if !ok {
			return c.Status(fiber.StatusInternalServerError).SendString("User ID is not a string")
		}
		exists, err := dbInstance.CheckUserExistsByID(idStr)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user by ID")
		}

		if !exists {
			_, err := dbInstance.CreateUser(idStr, (*userCallback)["email"].(string), (*userCallback)["picture"].(string))
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Failed to create user: " + err.Error())
			}
			return c.SendString("ok")
		}

		// JSON Web Token Generation
		token, err := lib.GenerateJWT(idStr, svrCfg.JWTTokenSecret)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate JWT: " + err.Error())
		}
		c.Cookie(&fiber.Cookie{
			Name:     "jwt",
			Value:    *token,
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteStrictMode,
		})

		return c.SendString("ok")
	})

	auth := api.Group("/", Requirelogin(db, svrCfg.JWTTokenSecret))

	auth.Get("/user/me", func(c *fiber.Ctx) error {
		jwtToken := c.Cookies("jwt")
		if jwtToken == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("JWT token is required")
		}
		userID, err := lib.GetUserIDByJWT(jwtToken, svrCfg.JWTTokenSecret)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid JWT token: " + err.Error())
		}
		dbInstance := &structs.Database{DB: db}
		userDetail, err := dbInstance.GetUserDetailByID(userID)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user detail: " + err.Error())
		}
		return c.JSON(userDetail)
	})

	auth.Get("/user/:id", func(c *fiber.Ctx) error {
		userID := c.Params("id")
		dbInstance := &structs.Database{DB: db}
		userDetail, err := dbInstance.GetUserDetailByID(userID)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user detail: " + err.Error())
		}
		return c.JSON(userDetail)
	})

	auth.Post("/user/me/name", func(c *fiber.Ctx) error {
		newUserName := c.FormValue("name")
		jwtToken := c.Cookies("jwt")
		if jwtToken == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("JWT token is required")
		}
		userID, err := lib.GetUserIDByJWT(jwtToken, svrCfg.JWTTokenSecret)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid JWT token: " + err.Error())
		}
		dbInstance := &structs.Database{DB: db}
		userDetail, err := dbInstance.ChangeUserName(userID, newUserName)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to change user name: " + err.Error())
		}
		return c.JSON(userDetail)
	})

	auth.Get("/team/:id", func(c *fiber.Ctx) error {
		userID := c.Params("id")
		dbInstance := &structs.Database{DB: db}
		teamDetail, err := dbInstance.GetTeamDetailByID(userID)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get team detail: " + err.Error())
		}
		return c.JSON(teamDetail)
	})

	auth.Get("/geo", func(c *fiber.Ctx) error {
		dbInstance := &structs.Database{DB: db}
		geolocationDetails, err := dbInstance.GetGeolocationLatestAll()
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get geolocation details: " + err.Error())
		}
		return c.JSON(geolocationDetails)
	})

	auth.Post("/geo", func(c *fiber.Ctx) error {
		jwtToken := c.Cookies("jwt")
		if jwtToken == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("JWT token is required")
		}
		userID, err := lib.GetUserIDByJWT(jwtToken, svrCfg.JWTTokenSecret)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid JWT token: " + err.Error())
		}
		var requestData struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}
		if err := c.BodyParser(&requestData); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid request data: " + err.Error())
		}
		dbInstance := &structs.Database{DB: db}
		geolocation, err := dbInstance.AddGeolocation(userID, requestData.Latitude, requestData.Longitude)
		if err != nil {
			// Handle error
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to add geolocation: " + err.Error())
		}
		return c.JSON(geolocation)
	})

	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
