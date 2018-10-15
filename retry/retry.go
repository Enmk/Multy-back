package retry

import  (
	"fmt"
	"github.com/pkg/errors"
	"time"
	try "gopkg.in/matryer/try.v1"
)

type Retryable func() error
type LogFunc func(string, ...interface{})

type Retrier interface {
	Do(context string, retryable Retryable) error
}

type retrier struct {
	retriesCount int
	retryWait time.Duration
	logger LogFunc
}

func NewRetrier(retriesCount int, retryWait time.Duration, logger LogFunc) Retrier {
	return retrier{retriesCount, retryWait, logger}
}

func (self retrier) Do(context string, retryable Retryable) (error) {
	return try.Do(func(attempt int) (retry bool, err error) {
		retry = attempt < self.retriesCount
		defer func() {
			if r := recover(); r != nil {
				self.log("[%s] got panic on previous iteration: %v", context, r)
				err = errors.New(fmt.Sprintf("panic: %v", r))
			}
		}()

		self.log("%s... (attempt %d/%d)", context, attempt, self.retriesCount)
		err = retryable()
		if err != nil {
			self.log("%s attempt failed, will retry after waiting %v...", context, self.retryWait)

			if attempt < self.retriesCount - 1 {
				time.Sleep(self.retryWait)
			}
		}
		return
	})
}

func (self retrier) log(format string, args ...interface{}) {
	if self.logger != nil {
		self.logger(format, args...)
	}
}