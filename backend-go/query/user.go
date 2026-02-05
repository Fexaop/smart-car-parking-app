package query

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type UserQueries struct {
	DB *sql.DB
}

type User struct {
	ID       int    `json:"id"`
	GoogleID string `json:"google_id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture"`
}

func NewUserQueries(db *sql.DB) *UserQueries {
	return &UserQueries{DB: db}
}

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		google_id TEXT UNIQUE,
		email TEXT,
		name TEXT,
		picture TEXT
	);`
	
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	// Migrate existing table: add name and picture columns if they don't exist
	_, err = db.Exec("ALTER TABLE users ADD COLUMN name TEXT")
	if err != nil {
		// Column might already exist, ignore error
	}
	
	_, err = db.Exec("ALTER TABLE users ADD COLUMN picture TEXT")
	if err != nil {
		// Column might already exist, ignore error
	}

	return db, nil
}

func (uq *UserQueries) CreateOrIgnoreUser(googleID, email string) error {
	_, err := uq.DB.Exec(
		"INSERT OR IGNORE INTO users (google_id, email) VALUES (?, ?)",
		googleID, email,
	)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return err
	}
	return nil
}

func (uq *UserQueries) CreateOrUpdateUser(googleID, email, name, picture string) error {
	_, err := uq.DB.Exec(
		`INSERT INTO users (google_id, email, name, picture) VALUES (?, ?, ?, ?)
		 ON CONFLICT(google_id) DO UPDATE SET email=?, name=?, picture=?`,
		googleID, email, name, picture, email, name, picture,
	)
	if err != nil {
		log.Printf("Error creating/updating user: %v", err)
		return err
	}
	return nil
}

type UserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture,omitempty"`
}

func (uq *UserQueries) GetUserByEmail(email string) (UserInfo, error) {
	var user User
	err := uq.DB.QueryRow(
		"SELECT id, google_id, email, name, picture FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.GoogleID, &user.Email, &user.Name, &user.Picture)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return UserInfo{}, sql.ErrNoRows
		}
		return UserInfo{}, err
	}
	
	return UserInfo{
		ID:      user.GoogleID,
		Email:   user.Email,
		Name:    user.Name,
		Picture: user.Picture,
	}, nil
}

func (uq *UserQueries) GetUserByGoogleID(googleID string) (UserInfo, error) {
	var user User
	err := uq.DB.QueryRow(
		"SELECT id, google_id, email, name, picture FROM users WHERE google_id = ?",
		googleID,
	).Scan(&user.ID, &user.GoogleID, &user.Email, &user.Name, &user.Picture)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return UserInfo{}, sql.ErrNoRows
		}
		return UserInfo{}, err
	}
	
	return UserInfo{
		ID:      user.GoogleID,
		Email:   user.Email,
		Name:    user.Name,
		Picture: user.Picture,
	}, nil
}