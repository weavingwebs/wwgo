package wwgo

import (
	"fmt"
	"github.com/pkg/errors"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func PrintErrWithStack(err error) {
	fmt.Printf("Error: %s\n", err)
	if stackErr, ok := err.(stackTracer); ok {
		fmt.Println(stackErr.StackTrace())
	}
}
