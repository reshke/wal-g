package internal

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/wal-g/wal-g/internal/compression"
	"github.com/wal-g/wal-g/internal/crypto"
	"github.com/wal-g/wal-g/internal/crypto/openpgp"
	"github.com/wal-g/wal-g/internal/storages/storage"
	"github.com/wal-g/wal-g/internal/tracelog"
	"golang.org/x/time/rate"
	"os"
	"path/filepath"
)

const (
	DefaultDataBurstRateLimit = 8 * int64(DatabasePageSize)
	DefaultDataFolderPath     = "/tmp"
	WaleFileHost              = "file://localhost"
)

const MinAllowedConcurrency = 1

var DeprecatedExternalGpgMessage = fmt.Sprintf(
	`You are using deprecated functionality that uses an external gpg library.
It will be removed in next major version.
Please set GPG key using environment variables %s or %s.
`, PgpKeySetting, PgpKeyPathSetting)

type UnconfiguredStorageError struct {
	error
}

func NewUnconfiguredStorageError(storagePrefixVariants []string) UnconfiguredStorageError {
	return UnconfiguredStorageError{errors.Errorf("No storage is configured now, please set one of following settings: %v", storagePrefixVariants)}
}

func (err UnconfiguredStorageError) Error() string {
	return fmt.Sprintf(tracelog.GetErrorFormatter(), err.error)
}

type UnknownCompressionMethodError struct {
	error
}

func NewUnknownCompressionMethodError() UnknownCompressionMethodError {
	return UnknownCompressionMethodError{errors.Errorf("Unknown compression method, supported methods are: %v", compression.CompressingAlgorithms)}
}

func (err UnknownCompressionMethodError) Error() string {
	return fmt.Sprintf(tracelog.GetErrorFormatter(), err.error)
}

type UnsetRequiredSettingError struct {
	error
}

func NewUnsetRequiredSettingError(settingName string) UnsetRequiredSettingError {
	return UnsetRequiredSettingError{errors.Errorf("%v is required to be set, but it isn't", settingName)}
}

func (err UnsetRequiredSettingError) Error() string {
	return fmt.Sprintf(tracelog.GetErrorFormatter(), err.error)
}

type InvalidConcurrencyValueError struct {
	error
}

func NewInvalidConcurrencyValueError(concurrencyType string, value int) InvalidConcurrencyValueError {
	return InvalidConcurrencyValueError{errors.Errorf("%v value is expected to be positive but is: %v", concurrencyType, value)}
}

func (err InvalidConcurrencyValueError) Error() string {
	return fmt.Sprintf(tracelog.GetErrorFormatter(), err.error)
}

type UnmarshallingError struct {
	error
}

func NewUnmarshallingError(subject string, err error) UnmarshallingError {
	return UnmarshallingError{errors.Errorf("Failed to unmarshal %s: %v", subject, err)}
}

func (err UnmarshallingError) Error() string {
	return fmt.Sprintf(tracelog.GetErrorFormatter(), err.error)
}

// TODO : unit tests
func ConfigureLimiters() {
	if viper.IsSet(DiskRateLimitSetting) {
		diskLimit := viper.GetInt64(DiskRateLimitSetting)
		DiskLimiter = rate.NewLimiter(rate.Limit(diskLimit), int(diskLimit+DefaultDataBurstRateLimit)) // Add 8 pages to possible bursts
	}

	if viper.IsSet(NetworkRateLimitSetting) {
		netLimit := viper.GetInt64(NetworkRateLimitSetting)
		NetworkLimiter = rate.NewLimiter(rate.Limit(netLimit), int(netLimit+DefaultDataBurstRateLimit)) // Add 8 pages to possible bursts
	}
}

// TODO : unit tests
func ConfigureFolder() (storage.Folder, error) {
	skippedPrefixes := make([]string, 0)
	for _, adapter := range StorageAdapters {
		prefix, ok := GetWaleCompatibleSetting(adapter.prefixName)
		if !ok {
			skippedPrefixes = append(skippedPrefixes, "WALG_"+adapter.prefixName)
			continue
		}
		if adapter.prefixPreprocessor != nil {
			prefix = adapter.prefixPreprocessor(prefix)
		}

		settings, err := adapter.loadSettings()
		if err != nil {
			return nil, err
		}
		return adapter.configureFolder(prefix, settings)
	}
	return nil, NewUnconfiguredStorageError(skippedPrefixes)
}

// TODO : unit tests
func getDataFolderPath() string {
	var dataFolderPath string
	if !viper.IsSet(PgDataSetting) {
		dataFolderPath = DefaultDataFolderPath
	} else {
		pgdata := viper.GetString(PgDataSetting)
		dataFolderPath = filepath.Join(pgdata, "pg_wal")
		if _, err := os.Stat(dataFolderPath); err != nil {
			dataFolderPath = filepath.Join(pgdata, "pg_xlog")
			if _, err := os.Stat(dataFolderPath); err != nil {
				dataFolderPath = DefaultDataFolderPath
			}
		}
	}
	dataFolderPath = filepath.Join(dataFolderPath, "walg_data")
	return dataFolderPath
}

// TODO : unit tests
func configureWalDeltaUsage() (useWalDelta bool, deltaDataFolder DataFolder, err error) {
	useWalDelta = viper.GetBool(UseWalDeltaSetting)
	if !useWalDelta {
		return
	}
	dataFolderPath := getDataFolderPath()
	deltaDataFolder, err = NewDiskDataFolder(dataFolderPath)
	if err != nil {
		useWalDelta = false
		tracelog.WarningLogger.Printf("can't use wal delta feature because can't open delta data folder '%s'"+
			" due to error: '%v'\n", dataFolderPath, err)
		err = nil
	}
	return
}

// TODO : unit tests
func configureCompressor() (compression.Compressor, error) {
	compressionMethod := viper.GetString(CompressionMethodSetting)
	if _, ok := compression.Compressors[compressionMethod]; !ok {
		return nil, NewUnknownCompressionMethodError()
	}
	return compression.Compressors[compressionMethod], nil
}

// TODO : unit tests
func ConfigureLogging() error {
	if viper.IsSet(LogLevelSetting) {
		return tracelog.UpdateLogLevel(viper.GetString(LogLevelSetting))
	}
	return nil
}

func getArchiveDataFolderPath() (path string) {
	if !viper.IsSet(PgDataSetting) {
		path = DefaultDataFolderPath
	} else {
		pgdata := viper.GetString(PgDataSetting)
		path = filepath.Join(pgdata, "pg_wal")
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(pgdata, "pg_xlog")
			if _, err := os.Stat(path); err != nil {
				path = DefaultDataFolderPath
			}
		}
	}
	path = filepath.Join(path, "walg_archive_status")

	return
}

// TODO : unit tests
func configureArchiveStatusManager() (archiveDataFolder DataFolder, err error){
	archiveFolderPath := getArchiveDataFolderPath()

	archiveDataFolder, err = NewDiskDataFolder(archiveFolderPath)

	return
}


// ConfigureUploader connects to storage and creates an uploader. It makes sure
// that a valid session has started; if invalid, returns AWS error
// and `<nil>` values.
func ConfigureUploader() (uploader *Uploader, err error) {
	folder, err := ConfigureFolder()
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure folder")
	}

	compressor, err := configureCompressor()
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure compression")
	}

	useWalDelta, deltaDataFolder, err := configureWalDeltaUsage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure WAL Delta usage")
	}

	var deltaFileManager *DeltaFileManager = nil
	if useWalDelta {
		deltaFileManager = NewDeltaFileManager(deltaDataFolder)
	}

	archiveStatusManager, err := configureArchiveStatusManager()

	if err != nil {
		return nil, errors.Wrap(err, "failed to configure Archive Status Manager")
	}

	uploader = NewUploader(compressor, folder, deltaFileManager, archiveStatusManager)

	return uploader, err
}

// ConfigureCrypter uses environment variables to create and configure a crypter.
// In case no configuration in environment variables found, return `<nil>` value.
func ConfigureCrypter() crypto.Crypter {
	loadPassphrase := func() (string, bool) {
		return GetSetting(PgpKeyPassphraseSetting)
	}

	// key can be either private (for download) or public (for upload)
	if viper.IsSet(PgpKeySetting) {
		return openpgp.CrypterFromKey(viper.GetString(PgpKeySetting), loadPassphrase)
	}

	// key can be either private (for download) or public (for upload)
	if viper.IsSet(PgpKeyPathSetting) {
		return openpgp.CrypterFromKeyPath(viper.GetString(PgpKeyPathSetting), loadPassphrase)
	}

	if keyRingID, ok := GetWaleCompatibleSetting(GpgKeyIDSetting); ok {
		tracelog.WarningLogger.Printf(DeprecatedExternalGpgMessage)
		return openpgp.CrypterFromKeyRingID(keyRingID, loadPassphrase)
	}

	return nil
}

func GetMaxDownloadConcurrency() (int, error) {
	return GetMaxConcurrency(DownloadConcurrencySetting)
}

func GetMaxUploadConcurrency() (int, error) {
	return GetMaxConcurrency(UploadConcurrencySetting)
}

// This setting is intentionally undocumented in README. Effectively, this configures how many prepared tar Files there
// may be in uploading state during backup-push.
func GetMaxUploadQueue() (int, error) {
	return GetMaxConcurrency(UploadQueueSetting)
}

func GetMaxUploadDiskConcurrency() (int, error) {
	return GetMaxConcurrency(UploadDiskConcurrencySetting)
}

func GetMaxConcurrency(concurrencyType string) (int, error) {
	concurrency := viper.GetInt(concurrencyType)

	if concurrency < MinAllowedConcurrency {
		return MinAllowedConcurrency, NewInvalidConcurrencyValueError(concurrencyType, concurrency)
	}
	return concurrency, nil
}

func GetSentinelUserData() interface{} {
	dataStr, ok := GetSetting(SentinelUserDataSetting)
	if !ok || len(dataStr) == 0 {
		return nil
	}
	var out interface{}
	err := json.Unmarshal([]byte(dataStr), &out)
	if err != nil {
		tracelog.WarningLogger.PrintError(NewUnmarshallingError(SentinelUserDataSetting, err))
		return dataStr
	}
	return out
}
