package cmd

import "github.com/spf13/cobra"

var startCmd = &cobra.Command{
	Use:	"start",
	Short:	"Start the local server in the background",
	Args: 	cobra.ExactArgs(1),
	Run: 	StartServer,	
}

func StartServer(cmd *cobra.Command, args []string){
	username := GetUSername
}

func init(){
	rootCmd.AddCommand(startCmd)
}