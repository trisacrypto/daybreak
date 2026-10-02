package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"

	confire "go.rtnl.ai/confire/usage"

	"github.com/trisacrypto/daybreak/pkg"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/config"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/server"
)

func main() {
	// Load environment variables from .env file
	_ = godotenv.Load()

	app := &cli.Command{
		Name:    "daybreak",
		Usage:   "start and manage the daybreak server",
		Version: pkg.Version(false),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "dsn",
				Usage:   "for database operations, use this DSN instead of the configured one",
				Sources: cli.EnvVars("DAYBREAK_DATABASE_URL"),
			},
		},
		Commands: []*cli.Command{
			{
				Name:     "serve",
				Usage:    "start the daybreak server",
				Action:   serve,
				Category: "service",
			},
			{
				Name:     "config",
				Usage:    "print daybreak configuration guide",
				Category: "service",
				Action:   usage,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "list",
						Aliases: []string{"l"},
						Usage:   "print in list mode instead of table mode",
					},
				},
			},
			{
				Name:      "load",
				Usage:     "load data from a file into the database",
				Action:    load,
				ArgsUsage: "daybreak.json",
				Category:  "service",
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		if ec, ok := err.(cli.ExitCoder); ok {
			os.Exit(ec.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

//===========================================================================
// Server commands
//===========================================================================

// serve starts the endeavor HTTP server.
func serve(ctx context.Context, cmd *cli.Command) (err error) {
	var srv *server.Server
	if srv, err = server.New(); err != nil {
		return cli.Exit(err, 1)
	}
	if err = srv.Serve(); err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

// usage prints the endeavor configuration guide.
func usage(ctx context.Context, cmd *cli.Command) error {
	tabs := tabwriter.NewWriter(os.Stdout, 1, 0, 4, ' ', 0)
	format := confire.DefaultTableFormat
	if cmd.Bool("list") {
		format = confire.DefaultListFormat
	}

	var conf config.Config
	if err := confire.Usagef(config.Prefix, &conf, tabs, format); err != nil {
		return cli.Exit(err, 1)
	}
	tabs.Flush()
	return nil
}

// load loads data from a file into the database.
func load(ctx context.Context, cmd *cli.Command) (err error) {
	if cmd.NArg() != 1 {
		return cli.Exit("specify only one path to a daybreak file to load", 1)
	}

	filename := cmd.Args().First()
	if filename == "" {
		return cli.Exit("a daybreak fixture file is required", 1)
	}

	var conf config.Config
	if conf, err = config.Get(); err != nil {
		return cli.Exit(err, 1)
	}

	if conf.DatabaseURL == "" {
		return cli.Exit("a database URL is required to load data", 1)
	}

	var conn *db.DB
	if conn, err = db.Open(conf.DatabaseDSN()); err != nil {
		return cli.Exit(err, 1)
	}
	defer conn.Close()

	var records []*api.GDSRecord
	if records, err = api.Fixture(filename); err != nil {
		return cli.Exit(err, 1)
	}

	var tx *db.Tx
	if tx, err = conn.Begin(ctx, &sql.TxOptions{ReadOnly: false}); err != nil {
		return cli.Exit(err, 1)
	}
	defer tx.Rollback()

	for _, record := range records {
		company := record.Model()
		if err = tx.CreateCompany(company); err != nil {
			return cli.Exit(err, 1)
		}

		target := strings.TrimPrefix(record.Endpoint, "mailto:")

		for i, contact := range record.Contacts {
			contact := contact.Model()
			contact.CompanyID = company.ID
			if err = tx.CreateContact(contact); err != nil {
				return cli.Exit(err, 1)
			}

			if contact.Email == target || i == 0 {
				company.PrimaryContact = uuid.NullUUID{UUID: contact.ID, Valid: true}
			}
		}

	}

	if err = tx.Commit(); err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}
