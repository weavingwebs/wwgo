package wwdb

import (
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"github.com/weavingwebs/wwgo"
)

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

func MigrateCommand(migrator func() *migrate.Migrate) *cli.Command {
	return &cli.Command{
		Name: "migrate",
		Subcommands: []*cli.Command{
			{
				Name: "up",
				Action: func(ctx *cli.Context) error {
					if err := migrator().Up(); err != nil {
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
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "yes",
						Aliases: []string{"y"},
					},
					&cli.IntFlag{
						Name:  "steps",
						Value: 1,
					},
				},
				Action: func(ctx *cli.Context) error {
					if !ctx.Bool("yes") && !wwgo.CliConfirm("Are you sure you want to apply 1 down migration?") {
						fmt.Println("cancelled")
						return nil
					}
					if err := migrator().Steps(-ctx.Int("steps")); err != nil {
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
					if err := migrator().Force(ctx.Int("version")); err != nil {
						return err
					}
					fmt.Println("👍️")
					return nil
				},
			},
			{
				Name: "drop",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "yes",
						Aliases: []string{"y"},
					},
				},
				Action: func(ctx *cli.Context) error {
					if !ctx.Bool("yes") && !(wwgo.CliConfirm("Are you sure you want to destroy the whole database?") && wwgo.CliConfirm("Seriously, you are really sure you want to destroy the whole database?")) {
						fmt.Println("cancelled")
						return nil
					}
					if err := migrator().Drop(); err != nil {
						return err
					}
					fmt.Println("👍️")
					return nil
				},
			},
		},
	}
}
