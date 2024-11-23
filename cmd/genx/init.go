package main

import (
	"context"
	"log"

	"github.com/molon/genx/pkg/configx"
	"github.com/molon/genx/starter"
	"github.com/spf13/cobra"
)

var initConfLoader configx.Loader[*starter.Config]

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Init project with starter boilerplate",
	Run: func(cmd *cobra.Command, args []string) {
		confPath, err := cmd.Flags().GetString("config")
		if err != nil {
			log.Fatalf("Failed to get config path: %v", err)
		}

		conf, err := initConfLoader(confPath)
		if err != nil {
			log.Fatalf("Failed to load config: %+v", err)
		}

		if err := starter.Extract(context.Background(), conf); err != nil {
			log.Fatalf("Failed to extract starter boilerplate: %+v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().String("config", "", "path to the configuration yaml file")

	loader, err := starter.InitializeConfig(initCmd.Flags(), "")
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}
	initConfLoader = loader
}
