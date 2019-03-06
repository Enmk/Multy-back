package storage

import "github.com/pkg/errors"

type storageErrorContext interface {
	getErrorContext() string
}

func reportError(storage storageErrorContext, err error, message string) error {
	return errors.Wrapf(err, "DB error on %s : %s", storage.getErrorContext(), message)
}