package retry

import  (
	"fmt"
	"github.com/pkg/errors"
	"time"
	try "gopkg.in/matryer/try.v1"
	"github.com/jekabolt/slf"
)

type RetriableFunction func() error

type Retrier interface {
	Do(logger slf.Logger, retriable RetriableFunction) error
}

type retrier struct {
	retriesCount int
	retryWait time.Duration
}

func NewRetrier(retriesCount int, retryWait time.Duration) Retrier {
	return retrier{retriesCount, retryWait}
}

func (self retrier) Do(logger slf.Logger, retriable RetriableFunction) (error) {
	return try.Do(func(attempt int) (retry bool, err error) {
		retry = attempt < self.retriesCount
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("got panic: %v", r)
				err = errors.New(fmt.Sprintf("panic: %v", r))
			}
		}()

		logger.Infof("trying (attempt %d/%d)...", attempt, self.retriesCount)
		err = retriable()

		if err != nil {
			logger.Infof("attempt failed, will retry after waiting %v...", self.retryWait)

			if attempt < self.retriesCount - 1 {
				time.Sleep(self.retryWait)
			}
		}
		return
	})
}