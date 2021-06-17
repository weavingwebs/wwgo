package wwgo

import (
	"database/sql"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	sqldblogger "github.com/simukti/sqldb-logger"
	"github.com/simukti/sqldb-logger/logadapter/zerologadapter"
	"github.com/urfave/cli/v2"
	"strings"
	"time"
)

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

func MysqlDbMigrate(db *sqlx.DB, migrations *bindata.AssetSource) (*migrate.Migrate, error) {
	dbDriver, err := mysql.WithInstance(db.DB, &mysql.Config{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init migrate db driver")
	}

	dataDriver, err := bindata.WithInstance(migrations)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init migrate data driver")
	}

	migrator, err := migrate.NewWithInstance(
		"migrations",
		dataDriver,
		"db",
		dbDriver,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init migrate")
	}

	return migrator, nil
}

func MigrateCommand(migrator *migrate.Migrate) *cli.Command {
	return &cli.Command{
		Name: "migrate",
		Subcommands: []*cli.Command{
			{
				Name: "up",
				Action: func(ctx *cli.Context) error {
					if err := migrator.Up(); err != nil {
						if err != migrate.ErrNoChange {
							return err
						}
						fmt.Println(err)
					}
					fmt.Println("👍️")
					return nil
				},
			},
			{
				Name: "down",
				Action: func(ctx *cli.Context) error {
					if !CliConfirm("Are you sure you want to apply 1 down migration?") {
						fmt.Println("cancelled")
						return nil
					}
					if err := migrator.Steps(-1); err != nil {
						return err
					}
					fmt.Println("👍️")
					return nil
				},
			},
			{
				Name: "force",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:     "version",
						Required: true,
					},
				},
				Action: func(ctx *cli.Context) error {
					if err := migrator.Force(ctx.Int("version")); err != nil {
						return err
					}
					fmt.Println("👍️")
					return nil
				},
			},
		},
	}
}
