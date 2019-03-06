package storage

import (
	"github.com/pkg/errors"
	mgo "gopkg.in/mgo.v2"
)

type ErrorNotFound interface {
	error
}

type storageErrorContext interface {
	getErrorContext() string
}

func reportError(storage storageErrorContext, err error, message string) error {
	if err == mgo.ErrNotFound {
		return ErrorNotFound(errors.Wrapf(err, "DB error on %s : %s", storage.getErrorContext(), message))
	}

	return errors.Wrapf(err, "DB error on %s : %s", storage.getErrorContext(), message)
}