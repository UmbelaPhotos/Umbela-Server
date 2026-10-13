package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"umbela-server/internal/db"
	"umbela-server/internal/image"
	"umbela-server/internal/ingestion"
	"umbela-server/internal/storage"
	"umbela-server/internal/vault"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:	"add",
	Short:	"Add photos to your personal library",
	Args: 	cobra.ExactArgs(2),
	RunE:	RequireAuth(AddImages),
}

func AddImages(cmd *cobra.Command, args []string) error {
	ctx:=cmd.Context()
	userSecrets, err:=GetContextSecrets(ctx)
	if err!=nil{
		return err
	}

	addImagesPath, err:=filepath.Abs(args[1])
	if err!=nil{
		return fmt.Errorf("converting to absolute path: %w\n", err)
	}
	fmt.Printf("Processing images in path: %v\n", addImagesPath)

	userVault:=vault.NewVault(userSecrets.VaultPath, userSecrets.MasterKey, userSecrets.Salt)
	userVault.ReadConfigFile()

	tempDir:=filepath.Join(userVault.Path,"tmp")
	err=os.MkdirAll(tempDir,0700)
	if err!=nil{
		return err
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Created temporal dir for extraction: %v\n", tempDir)

	directoryFileGroups, err := ingestion.ScanDirectory(addImagesPath,4,tempDir)
	if err!=nil{
		return err
	}

	imageProcessor,err:=image.NewProcessor(userVault.Config.ThumbsDir, 320,0)
	if err!=nil{
		return err
	}
	fmt.Println("Initialized Govips")

	userDb, err:= db.InitDB(userSecrets.VaultPath, userSecrets.MasterKey)
	if err!=nil{
		return err
	}
	fmt.Println("Connected to db")

	photoRepo:=storage.NewPhotoRepository(userDb)

	ingestion.IngestPhotos(
		ctx, 
		directoryFileGroups, 
		imageProcessor, 
		photoRepo,
		userVault.Config.ImageDir,
		false,
		false,
		userSecrets.MasterKey,
	)

	return nil
}

func init() {
	rootCmd.AddCommand(addCmd)
}