package wwgo

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"runtime"
)

// stack is a direct copy from pkg/errors.
type stack []uintptr

// StackTrace is a direct copy from pkg/errors.
func (s *stack) StackTrace() errors.StackTrace {
	f := make([]errors.Frame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = errors.Frame((*s)[i])
	}
	return f
}

// callers is based on pkg/errors.
func callers() *stack {
	const depth = 8
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	var st stack = pcs[0:n]
	return &st
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func PrintErrWithStack(err error) {
	fmt.Printf("Error: %s\n", err)
	if stackErr, ok := err.(stackTracer); ok {
		fmt.Println(stackErr.StackTrace())
	}
}

// ClientError represents an error that should be displayed to the user.
type ClientError struct {
	message       string
	code          string
	originalError error
	stack         *stack
}

func (err *ClientError) GqlErrorCode() string {
	return err.code
}

func (err *ClientError) Error() string {
	return err.message
}

func (err *ClientError) Unwrap() error {
	return err.originalError
}

func (err *ClientError) StackTrace() errors.StackTrace {
	return err.stack.StackTrace()
}

func NewClientError(code string, message string, err error) *ClientError {
	return &ClientError{
		message:       message,
		code:          code,
		originalError: err,
		stack:         callers(),
	}
}

type ErrAndPanicGroup struct {
	gos *errgroup.Group
}

func NewErrAndPanicGroup() *ErrAndPanicGroup {
	return &ErrAndPanicGroup{
		gos: &errgroup.Group{},
	}
}

func NewErrAndPanicGroupWithContext(ctx context.Context) (*ErrAndPanicGroup, context.Context) {
	gos, ctx := errgroup.WithContext(ctx)
	return &ErrAndPanicGroup{
		gos: gos,
	}, ctx
}

func (g *ErrAndPanicGroup) Go(fn func() error) {
	g.gos.Go(func() (res error) {
		defer func() {
			if r := recover(); r != nil {
				res = errors.Errorf("caught panic: %v", r)
			}
		}()
		return fn()
	})
}

func (g *ErrAndPanicGroup) Wait() error {
	return g.gos.Wait()
}
