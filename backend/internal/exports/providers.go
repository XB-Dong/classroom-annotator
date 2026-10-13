package exports

import "errors"

var ErrMixedProviders = errors.New("MIXED_MEDIA_PROVIDERS")
var ErrRetired = errors.New("REMOTE_STORAGE_RETIRED")
var ErrBackfill = errors.New("MEDIA_BACKFILL_REQUIRED")
var ErrChanged = errors.New("MEDIA_CHANGED")

func providerOf(records []Record) (string, error) {
	if len(records) == 0 {
		return "", ErrEmpty
	}
	for _, record := range records {
		if record.Frame.StorageProvider == "mac" {
			return "", ErrRetired
		}
		if record.Frame.StorageProvider != "" && record.Frame.StorageProvider != "local" {
			return "", ErrInvalid
		}
	}
	return "local", nil
}
