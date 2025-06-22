package main

import (
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/m-tsuru/tenchi-geolocation/lib"
	"github.com/m-tsuru/tenchi-geolocation/structs"
	// "gorm.io/driver/sqlite"
	"gorm.io/driver/postgres"
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

func AllowTimingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 許可する時刻（"15:04" 形式）の配列
		allowedTimes := []string{
			"00:00",
			"00:30",
			"01:00",
			"01:30",
			"02:00",
			"02:30",
			"03:00",
			"03:30",
			"04:00",
			"04:30",
			"05:00",
			"05:30",
			"06:00",
			"06:30",
			"07:00",
			"07:30",
			"08:00",
			"08:30",
			"09:00",
			"09:30",
			"10:00",
			"10:30",
			"11:00",
			"11:30",
			"12:00",
			"12:30",
			"13:00",
			"13:30",
			"14:00",
			"14:30",
			"15:00",
			"15:30",
			"16:00",
			"16:30",
			"17:00",
			"17:30",
			"18:00",
			"18:30",
			"19:00",
			"19:30",
			"20:00",
			"20:30",
			"21:00",
			"21:30",
			"22:00",
			"22:30",
			"23:00",
			"23:30",
		}

		now := c.Context().Time()
		for _, t := range allowedTimes {
			parsed, err := strconv.ParseInt(t[:2], 10, 0)
			if err != nil {
				continue // フォーマット不正はスキップ
			}
			hour := int(parsed)
			minute, err := strconv.ParseInt(t[3:], 10, 0)
			if err != nil {
				continue // フォーマット不正はスキップ
			}
			// 指定時刻の前後2分の範囲
			startHM := hour*60 + int(minute) - 3
			endHM := hour*60 + int(minute) + 3

			// 現在時刻の年月日は無視し、時分のみ比較
			nowHM := now.Hour()*60 + now.Minute()

			if startHM <= nowHM && nowHM <= endHM {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).SendString("Request not allowed at this time")
	}
}

func main() {
	oaCfg, svrCfg, dsnCfg, webhookURL, err := lib.LoadConfig()
	if err != nil {
		// Handle error
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := gorm.Open(
		postgres.Open(*dsnCfg),
		&gorm.Config{},
	)
	if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }

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
	app.Static("/", "./web")

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
			return c.Redirect("/", fiber.StatusFound)
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

		return c.Redirect("/", fiber.StatusFound)
	})

	api.Post("/logout", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:     "jwt",
			Value:    "",
			HTTPOnly: true,
			Secure:   true,
			SameSite: fiber.CookieSameSiteStrictMode,
			Expires:  time.Unix(0, 0), // 1970年
		})
		return c.SendStatus(fiber.StatusOK)
	})

	auth := api.Group("/", Requirelogin(db, svrCfg.JWTTokenSecret))

	auth.Get("/user/me", func(c *fiber.Ctx) error {
		jwtToken := c.Cookies("jwt")
		if jwtToken == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("JWT token is required")
		}
		userID, err := lib.GetUserIDByJWT(jwtToken, svrCfg.JWTTokenSecret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid JWT token: " + err.Error())
		}
		dbInstance := &structs.Database{DB: db}
		userDetail, err := dbInstance.GetUserDetailByID(userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user detail: " + err.Error())
		}
		tid := strconv.Itoa(userDetail.Team.ID)
		teamDetail, err := dbInstance.GetTeamDetailByID(tid)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get team detail: " + err.Error())
		}
		return c.JSON(fiber.Map{
			"user_profile": userDetail.UserProfile,
			"team":         userDetail.Team,
			"team_members": teamDetail.Members,
		})
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
		var req struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid request data: " + err.Error())
		}
		jwtToken := c.Cookies("jwt")
		if jwtToken == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("JWT token is required")
		}
		userID, err := lib.GetUserIDByJWT(jwtToken, svrCfg.JWTTokenSecret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid JWT token: " + err.Error())
		}
		dbInstance := &structs.Database{DB: db}
		userDetail, err := dbInstance.ChangeUserName(userID, req.Name)
		if err != nil {
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

	auth.Post("/team/:id", func(c *fiber.Ctx) error {
		teamID := c.Params("id")
		var req struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid request data: " + err.Error())
		}
		dbInstance := &structs.Database{DB: db}
		var team structs.Team
		if err := dbInstance.First(&team, "id = ?", teamID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Team not found")
		}
		team.Name = req.Name
		if err := dbInstance.Save(&team).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to update team name: " + err.Error())
		}
		return c.JSON(team)
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

	auth.Post("/geo", AllowTimingMiddleware(), func(c *fiber.Ctx) error {
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
		ud, err := dbInstance.GetUserDetailByID(userID)
		if ud == nil {
			return c.Status(fiber.StatusInternalServerError).SendString("User detail not found")
		} else if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user detail: " + err.Error())
		}

		err = lib.NotifyGeolocationUpdate(ud, webhookURL, geolocation)
		if err != nil {
			log.Printf("Failed to notify geolocation update: %v", err)
		}
		return c.JSON(geolocation)
	})

	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
