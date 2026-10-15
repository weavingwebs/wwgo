package wwdb

import (
	"context"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"github.com/weavingwebs/wwgo"
	"io/fs"
)

func MysqlDbMigrate(db *sqlx.DB, migrations fs.FS, path string) (*migrate.Migrate, error) {
	dbDriver, err := mysql.WithInstance(db.DB, &mysql.Config{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init migrate db driver")
	}

	dataDriver, err := iofs.New(migrations, path)
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

func MigrateCommand(migrator func(ctx context.Context) *migrate.Migrate) *cli.Command {
	return &cli.Command{
		Name: "migrate",
		Commands: []*cli.Command{
			{
				Name: "up",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if err := migrator(ctx).Up(); err != nil {
						if !errors.Is(err, migrate.ErrNoChange) {
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
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if !cmd.Bool("yes") && !wwgo.CliConfirm(fmt.Sprintf("Are you sure you want to apply %d down migration(s)?", cmd.Int("steps"))) {
						fmt.Println("cancelled")
						return nil
					}
					if err := migrator(ctx).Steps(-cmd.Int("steps")); err != nil {
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
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if err := migrator(ctx).Force(cmd.Int("version")); err != nil {
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
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if !cmd.Bool("yes") && !(wwgo.CliConfirm("Are you sure you want to destroy the whole database?") && wwgo.CliConfirm("Seriously, you are really sure you want to destroy the whole database?")) {
						fmt.Println("cancelled")
						return nil
					}
					if err := migrator(ctx).Drop(); err != nil {
						return err
					}
					fmt.Println("👍️")
					return nil
				},
			},
		},
	}
}
