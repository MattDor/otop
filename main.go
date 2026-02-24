package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mdoeren/otop/internal/db"
	"github.com/mdoeren/otop/internal/ui"
)

func main() {
	connStr := flag.String("conn", "", "Oracle connection string (user/password@host:port/service)")
	flag.Parse()

	if *connStr == "" {
		fmt.Fprintln(os.Stderr, "error: -conn is required")
		flag.Usage()
		os.Exit(1)
	}

	database, err := db.Connect(*connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not connect to database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	app := ui.NewApp(database)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
