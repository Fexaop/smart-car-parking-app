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
		email TEXT
	);`
	
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
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

func (uq *UserQueries) GetUserByGoogleID(googleID string) (*User, error) {
	var user User
	err := uq.DB.QueryRow(
		"SELECT id, google_id, email FROM users WHERE google_id = ?",
		googleID,
	).Scan(&user.ID, &user.GoogleID, &user.Email)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, err
	}
	
	return &user, nil
}