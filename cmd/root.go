/*
Copyright © 2025 Marco Souza <marco@tremtec.com>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "djc",
	Short: "A CLI toolbox for DJing",
	Long:  `djc is a CLI tool for DJs to download and manage music from YouTube.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
