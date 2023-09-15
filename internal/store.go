package internal

import (
	"fmt"

	"database/sql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"awesomeProject/models"
)

// Database represents a PostgreSQL database connection.
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new Database object with the given connection parameters.
func NewDatabase(host, port, user, password, dbname string) (*Database, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// RunMigrations runs database migrations from the specified directory.
func (d *Database) RunMigrations(migrationDir string) error {
	driver, err := postgres.WithInstance(d.db, &postgres.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationDir),
		"postgres", driver)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

// Upsert either updates an existing ETF record or creates a new one.
func (d *Database) Upsert(etf models.ETF) error {
	// Use a transaction to ensure atomicity
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	// Check if the ETF with the given ID exists
	var count int
	err = tx.QueryRow("SELECT COUNT(*) FROM etfs WHERE id = $1", etf.ID).Scan(&count)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if count > 0 {
		// Update the existing ETF
		_, err = tx.Exec("UPDATE etfs SET data = $1, updated_at = NOW() WHERE id = $2", etf.Data, etf.ID)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	} else {
		// Insert a new ETF
		_, err = tx.Exec("INSERT INTO etfs (id, data, created_at, updated_at) VALUES ($1, $2, NOW(), NOW())", etf.ID, etf.Data)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

// GetAllIDs retrieves all available ETF IDs from the database.
func (d *Database) GetAllIDs() ([]string, error) {
	rows, err := d.db.Query("SELECT id FROM etfs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// GetByID retrieves an ETF by its ID.
func (d *Database) GetByID(id string) (*models.ETF, error) {
	var etf models.ETF

	// Query the database by ID and scan the result into the etf variable
	err := d.db.QueryRow("SELECT * FROM etfs WHERE id = $1", id).Scan(
		&etf.ID,
		&etf.Data,
		&etf.CreatedAt,
		&etf.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Handle the case where no rows match the given ID
			return nil, fmt.Errorf("ETF not found")
		}
		// Handle other errors
		return nil, err
	}

	return &etf, nil
}

// UserExists checks if a user with the given username and password exists in the database.
func (d *Database) UserExists(username, password string) (bool, error) {
	// Query the database to check if the user exists
	var exists bool
	err := d.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND password = $2)", username, password).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
