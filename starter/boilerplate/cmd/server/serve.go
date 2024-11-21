package main

import (
	"log"

	"github.com/molon/genx/pkg/configx"
	"github.com/molon/genx/starter/boilerplate/server"
	"github.com/molon/genx/starter/boilerplate/server/config"
	"github.com/spf13/cobra"
)

var serveConfLoader configx.Loader[*config.Config]

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the server",
	Run: func(cmd *cobra.Command, args []string) {
		confPath, err := cmd.Flags().GetString("config")
		if err != nil {
			log.Fatalf("Failed to get config path: %v", err)
		}

		conf, err := serveConfLoader(confPath)
		if err != nil {
			log.Fatalf("Failed to load config: %+v", err)
		}

		if err := server.Serve(conf); err != nil {
			log.Fatalf("Failed to serve server: %+v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().String("config", "", "path to the configuration yaml file")

	loader, err := config.Initialize(serveCmd.Flags(), "")
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}
	serveConfLoader = loader
}
