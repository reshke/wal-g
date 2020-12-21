package mysql

import (
	"github.com/spf13/cobra"
	"github.com/wal-g/tracelog"
	"github.com/wal-g/wal-g/internal"
	"github.com/wal-g/wal-g/internal/databases/mysql"
)

const (
	BackupMarkShortDescription = "Marks a backup permanent or impermanent"
	BackupMarkLongDescription  = `Marks a backup permanent by default, or impermanent when flag is provided.
	Permanent backups are prevented from being removed when running delete.`
	ImpermanentDescription = "Marks a backup impermanent"
	ImpermanentFlagShortHand        = "i"
	ImpermanentFlag        = "impermanent"
)

var (
	// backupMarkCmd represents the backupMark command
	backupMarkCmd = &cobra.Command{
		Use:   "backup-mark backup_name",
		Short: BackupMarkShortDescription,
		Long:  BackupMarkLongDescription,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			uploader, err := internal.ConfigureWalUploader()
			tracelog.ErrorLogger.FatalOnError(err)
			mysql.NewMysqlMarkBackup().HandleBackupMark( uploader.Uploader, args[0], !toImpermanent)
		},
	}
	toImpermanent = false
)

func init() {
	backupMarkCmd.Flags().BoolVarP(&toImpermanent, ImpermanentFlag, ImpermanentFlagShortHand, false, ImpermanentDescription)
	cmd.AddCommand(backupMarkCmd)
}
