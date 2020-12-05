package mysql

import (
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wal-g/storages/storage"
	"github.com/wal-g/tracelog"
	"github.com/wal-g/wal-g/internal"
	"github.com/wal-g/wal-g/internal/databases/mysql"
	"github.com/wal-g/wal-g/utility"
)

var confirmed = false

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete", //for example "delete mysql before time"
	Short: "Clears old backups and binlogs",
}

var deleteBeforeCmd = &cobra.Command{
	Use:     internal.DeleteBeforeUsageExample, // TODO : improve description
	Example: internal.DeleteBeforeExamples,
	Args:    internal.DeleteBeforeArgsValidator,
	Run:     runDeleteBefore,
}

var deleteRetainCmd = &cobra.Command{
	Use:       internal.DeleteRetainUsageExample, // TODO : improve description
	Example:   internal.DeleteRetainExamples,
	ValidArgs: internal.StringModifiers,
	Args:      internal.DeleteRetainArgsValidator,
	Run:       runDeleteRetain,
}

var deleteEverythingCmd = &cobra.Command{
	Use:       internal.DeleteEverythingUsageExample, // TODO : improve description
	Example:   internal.DeleteEverythingExamples,
	ValidArgs: internal.StringModifiersDeleteEverything,
	Args:      internal.DeleteEverythingArgsValidator,
	Run:       runDeleteEverything,
}

func runDeleteEverything(cmd *cobra.Command, args []string) {
	folder, err := internal.ConfigureFolder()
	tracelog.ErrorLogger.FatalOnError(err)
	internal.DeleteEverything(folder, confirmed, args)
}

func runDeleteBefore(cmd *cobra.Command, args []string) {
	folder, err := internal.ConfigureFolder()
	tracelog.ErrorLogger.FatalOnError(err)
	isFullBackup := func(object storage.Object) bool {
		return IsFullBackup()
	}
	backups, err := internal.GetBackupSentinelObjects(folder)
	tracelog.ErrorLogger.FatalOnError(err)
	internal.HandleDeleteBefore(folder, backups, args, confirmed, isFullBackup, makeLessFunc(folder), getBackupTime)
}

func runDeleteRetain(cmd *cobra.Command, args []string) {
	folder, err := internal.ConfigureFolder()
	tracelog.ErrorLogger.FatalOnError(err)
	isFullBackup := func(object storage.Object) bool {
		return IsFullBackup()
	}
	backups, err := internal.GetBackupSentinelObjects(folder)
	tracelog.ErrorLogger.FatalOnError(err)
	internal.HandleDeleteRetain(folder, backups, args, confirmed, isFullBackup, makeLessFunc(folder))
}

func init() {
	cmd.AddCommand(deleteCmd)
	deleteCmd.AddCommand(deleteBeforeCmd, deleteRetainCmd, deleteEverythingCmd)
	deleteCmd.PersistentFlags().BoolVar(&confirmed, internal.ConfirmFlag, false, "Confirms backup deletion")
}

func IsFullBackup() bool {
	return true
}

func makeLessFunc(folder storage.Folder) func(object1, object2 storage.Object) bool {
	return func(object1, object2 storage.Object) bool {
		time1, ok := utility.TryFetchTimeRFC3999(object1.GetName())
		if !ok {
			return binlogLess(folder, object1, object2)
		}
		time2, ok := utility.TryFetchTimeRFC3999(object2.GetName())
		if !ok {
			return binlogLess(folder, object1, object2)
		}
		return time1 < time2
	}
}

func binlogLess(folder storage.Folder, object1, object2 storage.Object) bool {
	binlogName1, ok := tryFetchBinlogName(folder, object1)
	if !ok {
		return false
	}
	binlogName2, ok := tryFetchBinlogName(folder, object2)
	if !ok {
		return false
	}
	return binlogName1 < binlogName2
}

func tryFetchBinlogName(folder storage.Folder, object storage.Object) (string, bool) {
	name := object.GetName()
	if strings.HasPrefix(name, mysql.BinlogPath) {
		_, name = path.Split(name)
		return name, true
	}
	name = strings.Replace(name, utility.SentinelSuffix, "", 1)
	baseBackupFolder := folder.GetSubFolder(utility.BaseBackupPath)
	backup := internal.NewBackup(baseBackupFolder, name)
	var sentinel mysql.StreamSentinelDto
	err := internal.FetchStreamSentinel(backup, &sentinel)
	if err != nil {
		tracelog.InfoLogger.Println("Fail to fetch stream sentinel " + name)
		return "", false
	}
	return sentinel.BinLogStart, true
}

func getBackupTime(object storage.Object) time.Time {
	return object.GetLastModified()
}
