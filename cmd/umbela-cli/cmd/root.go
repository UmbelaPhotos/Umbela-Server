package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:	"umbela",
	Short:	"Umbela is a self-hosted photo server focus on Privacy & Security",
	Long:	"Umbela is a personal and secure server that manages photos with local encryption and accessed through Umbela App",
}

func Execute() error{
	return rootCmd.Execute()
}
