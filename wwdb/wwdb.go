package wwdb

import (
	"database/sql"
	"github.com/cenkalti/backoff/v4"
	mysql2 "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	sqldblogger "github.com/simukti/sqldb-logger"
	"github.com/simukti/sqldb-logger/logadapter/zerologadapter"
	"os"
	"strings"
	"time"
)

const MB = 1 << 20

func OpenDb(log zerolog.Logger, driverName string, dsn string, maxOpenConns int) (*sqlx.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, errors.Wrap(err, "Error opening connection to DB")
	}
	db.SetMaxOpenConns(maxOpenConns)

	// Wrap the connection with a logger.
	db = sqldblogger.OpenDriver(
		dsn,
		db.Driver(),
		zerologadapter.New(log),
		sqldblogger.WithPreparerLevel(sqldblogger.LevelTrace),
		sqldblogger.WithQueryerLevel(sqldblogger.LevelDebug),
		sqldblogger.WithExecerLevel(sqldblogger.LevelTrace),
	)

	// Wrap with SQLX & override the name mapper.
	sqlx.NameMapper = func(str string) string {
		return strings.ToLower(string(str[0])) + str[1:]
	}
	dbX := sqlx.NewDb(db, driverName)

	// Check connection and return.
	err = backoff.RetryNotify(
		db.Ping,
		backoff.NewExponentialBackOff(),
		func(err error, duration time.Duration) {
			log.Error().Msgf("Failed to connect to database, retrying in %s...", duration)
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to database")
	}
	return dbX, nil
}

func OpenDbFromWhaleblazer(log zerolog.Logger, maxOpenConns int) (*sqlx.DB, error) {
	sqlConfig, err := WhaleblazerMysqlConfig()
	if err != nil {
		return nil, err
	}
	return OpenDb(log, "mysql", sqlConfig.FormatDSN(), maxOpenConns)
}

func WhaleblazerMysqlConfig() (*mysql2.Config, error) {
	dbHost := os.Getenv("WHALEBLAZER_DB_HOST")
	if dbHost == "" {
		return nil, errors.Errorf("WHALEBLAZER_DB_HOST is not set")
	}
	dbName := os.Getenv("WHALEBLAZER_DB_NAME")
	if dbName == "" {
		return nil, errors.Errorf("WHALEBLAZER_DB_NAME is not set")
	}
	dbUser := os.Getenv("WHALEBLAZER_DB_USER")
	if dbUser == "" {
		return nil, errors.Errorf("WHALEBLAZER_DB_USER is not set")
	}
	dbPass := os.Getenv("WHALEBLAZER_DB_PASS")
	if dbPass == "" {
		return nil, errors.Errorf("WHALEBLAZER_DB_PASS is not set")
	}
	dbPort := os.Getenv("WHALEBLAZER_DB_PORT")
	if dbPort == "" {
		dbPort = "3306"
	}
	dbCollation := os.Getenv("WHALEBLAZER_DB_COLLATION")

	sqlConfig := &mysql2.Config{
		User:                 dbUser,
		Passwd:               dbPass,
		Net:                  "tcp",
		Addr:                 dbHost + ":" + dbPort,
		DBName:               dbName,
		Collation:            dbCollation,
		Loc:                  time.UTC,
		MaxAllowedPacket:     4 * MB,
		AllowNativePasswords: true,
		MultiStatements:      true,
		ParseTime:            true,
	}
	return sqlConfig, nil
}
