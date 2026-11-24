package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"

	confire "go.rtnl.ai/confire/usage"
	"go.rtnl.ai/x/dsn"

	"github.com/trisacrypto/daybreak/pkg"
	"github.com/trisacrypto/daybreak/pkg/config"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/ops"
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
				Name:     "session-key",
				Usage:    "generate a new session key",
				Action:   sessionKey,
				Category: "service",
			},
			{
				Name:      "import",
				Usage:     "import data from a zip file into the database",
				ArgsUsage: "daybreak.zip",
				Action:    importDaybreak,
				Category:  "service",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "dsn",
						Usage:   "DSN to use for the database",
						Sources: cli.EnvVars("DAYBREAK_DATABASE_URL"),
					},
				},
			},
			{
				Name:      "export",
				Usage:     "export data from the database to a zip file",
				ArgsUsage: "daybreak.zip",
				Action:    exportDaybreak,
				Category:  "service",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "dsn",
						Usage:   "DSN to use for the database",
						Sources: cli.EnvVars("DAYBREAK_DATABASE_URL"),
					},
				},
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

// sessionKey generates a new session key.
func sessionKey(ctx context.Context, cmd *cli.Command) (err error) {
	out := make([]byte, 32)
	if _, err = rand.Read(out); err != nil {
		return cli.Exit(err, 1)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(out))
	return nil
}

func importDaybreak(ctx context.Context, cmd *cli.Command) (err error) {
	if cmd.NArg() != 1 {
		return cli.Exit("specify a single path to the zip file to import", 1)
	}

	var path string
	if path = cmd.Args().First(); path == "" {
		return cli.Exit("specify a path to the zip file to import", 1)
	}

	var db *db.DB
	if db, err = openDB(cmd); err != nil {
		return cli.Exit(err, 1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	if err = ops.Import(ctx, db, path); err != nil {
		return cli.Exit(err, 1)
	}

	return nil
}

func exportDaybreak(ctx context.Context, cmd *cli.Command) (err error) {
	if cmd.NArg() != 1 {
		return cli.Exit("specify a single path to the zip file to import", 1)
	}

	var path string
	if path = cmd.Args().First(); path == "" {
		return cli.Exit("specify a path to the zip file to import", 1)
	}

	var db *db.DB
	if db, err = openDB(cmd); err != nil {
		return cli.Exit(err, 1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	if err = ops.Export(ctx, db, path); err != nil {
		return cli.Exit(err, 1)
	}

	return nil
}

func openDB(cmd *cli.Command) (conn *db.DB, err error) {
	var uri string
	if uri = cmd.String("dsn"); uri == "" {
		return nil, cli.Exit("specify a DSN to use for the database", 1)
	}

	var dsi *dsn.DSN
	if dsi, err = dsn.Parse(uri); err != nil {
		return nil, err
	}

	if conn, err = db.Open(dsi); err != nil {
		return nil, err
	}
	return conn, nil
}
