package structs

import (
	"time"

	"gorm.io/gorm"
)

type Database struct {
	*gorm.DB
}

type User struct {
	ID        string `gorm:"primaryKey"`
	Email     string
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	IsExist   bool      `gorm:"default:true"`
}

type UserProfile struct {
	ID        string    `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	UserName  string    `gorm:"not null"`
	TeamID    int       `gorm:"not null"`
	AvatarURL string    `gorm:"default:null"`
}

type Team struct {
	ID        int       `gorm:"primaryKey,autoIncrement"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	Name      string    `gorm:"unique;not null"`
}

type Geolocation struct {
	ID        int       `gorm:"primaryKey,autoIncrement"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	UserID    string    `gorm:"not null"`
	Latitude  float64   `gorm:"not null"`
	Longitude float64   `gorm:"not null"`
}

type UserDetail struct {
	UserProfile UserProfile
	Team        Team
}

type TeamDetail struct {
	Team    Team
	Members []UserProfile
}

type GeolocationDetail struct {
	TeamDetail  TeamDetail
	Geolocation Geolocation
}

func (db *Database) AutoMigrateModels() error {
	err := db.AutoMigrate(&User{}, &UserProfile{}, &Team{}, &Geolocation{})
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) AutoCreateTestData() error {
	var count int64
	db.Model(&Team{}).Where("id = ?", 9).Count(&count)
	if count == 0 {
		testTeam := Team{
			ID:   9,
			Name: "チーム未設定",
		}
		if err := db.Create(&testTeam).Error; err != nil {
			return err
		}
	}

	db.Model(&Geolocation{}).Where("id = ?", 1).Count(&count)
	if count == 0 {
		testGeolocation := Geolocation{
			ID:        1,
			UserID:    "111862085249385638299",
			Latitude:  34.385973,
			Longitude: 132.453895,
		}
		if err := db.Create(&testGeolocation).Error; err != nil {
			return err
		}
	}

	return nil
}

func (db *Database) CheckUserExistsByID(id string) (bool, error) {
	var count int64
	if err := db.Model(&User{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *Database) GetUserByID(userID string) (*User, error) {
	var user User
	if err := db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *Database) GetUserDetailByID(userID string) (*UserDetail, error) {
	var user User
	if err := db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	var userProfile UserProfile
	if err := db.First(&userProfile, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	var team Team
	if err := db.First(&team, "id = ?", userProfile.TeamID).Error; err != nil {
		return nil, err
	}

	return &UserDetail{
		UserProfile: userProfile,
		Team:        team,
	}, nil
}

func (db *Database) GetTeamDetailByID(teamID string) (*TeamDetail, error) {
	var team Team
	if err := db.First(&team, "id = ?", teamID).Error; err != nil {
		return nil, err
	}

	var members []UserProfile
	if err := db.Where("team_id = ?", teamID).Find(&members).Error; err != nil {
		return nil, err
	}

	return &TeamDetail{
		Team:    team,
		Members: members,
	}, nil
}

func (db *Database) GetGeolocationLatestAll() (*[]GeolocationDetail, error) {
	var teams []Team
	if err := db.Find(&teams).Error; err != nil {
		return nil, err
	}

	var details []GeolocationDetail

	for _, team := range teams {
		var members []UserProfile
		if err := db.Where("team_id = ?", team.ID).Find(&members).Error; err != nil {
			return nil, err
		}

		// Collect all user IDs in this team
		var userIDs []string
		for _, member := range members {
			userIDs = append(userIDs, member.ID)
		}

		if len(userIDs) == 0 {
			continue // チームにメンバーがいない場合もスキップ
		}

		var latestGeolocation Geolocation
		// Get the latest geolocation for any user in this team
		if err := db.Where("user_id IN ?", userIDs).
			Order("created_at DESC").
			First(&latestGeolocation).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue // Geolocation が一件もない場合はスキップ
			}
			return nil, err
		}

		teamDetail := TeamDetail{
			Team:    team,
			Members: members,
		}

		details = append(details, GeolocationDetail{
			TeamDetail:  teamDetail,
			Geolocation: latestGeolocation,
		})
	}

	return &details, nil
}

func (db *Database) AddGeolocation(userID string, latitude float64, longitude float64) (*Geolocation, error) {
	geolocation := &Geolocation{
		UserID:    userID,
		Latitude:  latitude,
		Longitude: longitude,
	}
	if err := db.Create(geolocation).Error; err != nil {
		return nil, err
	}
	return geolocation, nil
}

func (db *Database) CreateUser(userID string, email string, picture string) (*User, error) {
	user := &User{
		ID:    userID,
		Email: email,
	}
	if err := db.Create(user).Error; err != nil {
		return nil, err
	}

	userProfile := &UserProfile{
		ID:        userID,
		UserName:  "ユーザ名未登録", // Default username, can be updated later
		TeamID:    9,         // Default team ID, can be updated later
		AvatarURL: picture,
	}
	if err := db.Create(userProfile).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (db *Database) ChangeUserName(userID string, newUserName string) (*UserProfile, error) {
	var userProfile UserProfile
	if err := db.First(&userProfile, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	userProfile.UserName = newUserName
	if err := db.Save(&userProfile).Error; err != nil {
		return nil, err
	}
	return &userProfile, nil
}
