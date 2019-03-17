package storage

import (
	"github.com/pkg/errors"
	mgo "gopkg.in/mgo.v2"
)

type ErrorNotFound interface {
	error
}

type errorContextProvider interface {
	getErrorContext() string
}

func reportError(storage errorContextProvider, err error, message string) error {
	if err == mgo.ErrNotFound {
		return ErrorNotFound(errors.Wrapf(err, "DB error on %s : %s", storage.getErrorContext(), message))
	}

	return errors.Wrapf(err, "DB error on %s : %s", storage.getErrorContext(), message)
}