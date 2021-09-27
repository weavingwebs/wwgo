package wwdb

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"os"
	"time"
)

type Flood struct {
	Db        *sqlx.DB
	Name      string
	Window    time.Duration
	Threshold int
}

type FloodHit struct {
	Event      string    `db:"event"`
	Identifier string    `db:"identifier"`
	Timestamp  time.Time `db:"timestamp"`
	Expiration time.Time `db:"expiration"`
}

func (f *Flood) Register(ctx context.Context, identifier string) {
	const q = `
	INSERT INTO flood (event, identifier, timestamp, expiration)
	VALUES (:event, :identifier, :timestamp, :expiration)
	`
	now := time.Now()
	_, err := f.Db.NamedExecContext(ctx, q, FloodHit{
		Event:      f.Name,
		Identifier: identifier,
		Timestamp:  now,
		Expiration: now.Add(f.Window),
	})
	if err != nil {
		panic(errors.Wrapf(err, "failed to insert into flood"))
	}
}

func (f *Flood) Clear(ctx context.Context, identifier string) error {
	const q = `DELETE FROM flood WHERE event = ? AND identifier = ?`
	_, err := f.Db.ExecContext(ctx, q, f.Name, identifier)
	if err != nil {
		return errors.Wrapf(err, "failed to delete from flood")
	}
	return nil
}

func (f *Flood) Empty(ctx context.Context) error {
	const q = `DELETE FROM flood WHERE event = ?`
	_, err := f.Db.ExecContext(ctx, q, f.Name)
	if err != nil {
		return errors.Wrapf(err, "failed to delete all from flood")
	}
	return nil
}

func (f *Flood) IsAllowed(ctx context.Context, identifier string) bool {
	const q = `SELECT COUNT(*) FROM flood WHERE event = ? AND identifier = ? AND timestamp > ?`
	var count int
	if err := f.Db.GetContext(ctx, &count, q, f.Name, identifier, time.Now().Add(-f.Window)); err != nil {
		panic(errors.Wrapf(err, "failed to query flood"))
	}
	return count < f.Threshold
}

func FloodGC(ctx context.Context, dbConn *sqlx.DB) (int64, error) {
	const q = `DELETE FROM flood WHERE expiration < ?`
	res, err := dbConn.ExecContext(ctx, q, time.Now())
	if err != nil {
		return 0, errors.Wrapf(err, "failed to GC flood")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		panic(errors.Wrapf(err, "failed to get rows affected"))
	}
	return affected, nil
}

func FloodUnban(ctx context.Context, dbConn *sqlx.DB, identifier string) (int64, error) {
	const q = `DELETE FROM flood WHERE identifier = ?`
	res, err := dbConn.ExecContext(ctx, q, identifier)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to delete from flood")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		panic(errors.Wrapf(err, "failed to get rows affected"))
	}
	return affected, nil
}

type FloodSummaryItem struct {
	Event      string    `db:"event"`
	Identifier string    `db:"identifier"`
	Count      int       `db:"count"`
	LastHit    time.Time `db:"lastHit"`
}

func FloodGetSummary(ctx context.Context, dbConn *sqlx.DB) ([]*FloodSummaryItem, error) {
	const q = `
	SELECT event, identifier, COUNT(*) as count, MAX(timestamp) as lastHit
	FROM flood
	WHERE expiration > ?
	GROUP BY event, identifier
	ORDER BY lastHit
	`

	var res []*FloodSummaryItem
	if err := dbConn.SelectContext(ctx, &res, q, time.Now()); err != nil {
		return nil, errors.Wrapf(err, "failed to select")
	}
	return res, nil
}

func FloodCommand(dbConn *sqlx.DB) *cli.Command {
	return &cli.Command{
		Name:  "flood",
		Usage: "Flood commands",
		Subcommands: []*cli.Command{
			{
				Name:  "get",
				Usage: "Get a summary of flood hits",
				Action: func(ctx *cli.Context) error {
					summary, err := FloodGetSummary(ctx.Context, dbConn)
					if err != nil {
						return err
					}

					if len(summary) == 0 {
						fmt.Printf("Flood table is empty\n")
						return nil
					}

					table := tablewriter.NewWriter(os.Stdout)
					table.SetHeader([]string{
						"Event",
						"Identifier",
						"Last Hit",
						"Count",
					})

					for _, item := range summary {
						table.Append([]string{
							item.Event,
							item.Identifier,
							item.LastHit.Format(time.RFC822),
							fmt.Sprintf("%d", item.Count),
						})
					}
					table.Render()
					return nil
				},
			},
			{
				Name:  "unban",
				Usage: "Unban an IP",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() < 1 {
						fmt.Println("Argument required: ip address")
						os.Exit(1)
					}
					ip := ctx.Args().Get(0)
					res, err := FloodUnban(ctx.Context, dbConn, ip)
					if err != nil {
						return err
					}
					fmt.Printf("%d entries deleted\n", res)
					return nil
				},
			},
		},
	}
}
