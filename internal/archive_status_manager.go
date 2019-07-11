package internal

import (
	"github.com/wal-g/wal-g/internal/tracelog"
	"path/filepath"
	"strings"
)

func getOnlyWalName(filename string) string {
	filename = filepath.Base(filename)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func isWalAlreadyUploaded(uploader *Uploader, walFilename string) bool {
	walFilename = getOnlyWalName(walFilename)
	if uploader.archiveStatusManager.FileExists(walFilename){
		tracelog.InfoLogger.Printf("wal file %s already uploaded, skipping it", walFilename)
		return true
	}
	return false
}

func markWalUploaded(uploader *Uploader, walFilename string) error {
	walFilename = getOnlyWalName(walFilename)
	tracelog.InfoLogger.Printf("mark wal %s as uploaded", walFilename)
	return uploader.archiveStatusManager.CreateFile(walFilename)
}

func unmarkWalFile(uploader *Uploader, walFilename string) error {
	walFilename = getOnlyWalName(walFilename)
	return uploader.archiveStatusManager.DeleteFile(walFilename)
}
