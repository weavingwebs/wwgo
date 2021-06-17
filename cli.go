package wwgo

import (
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"strings"
)

func CliAsk(question string, defaultAnswer string) string {
	fmt.Printf(question)
	if defaultAnswer != "" {
		fmt.Printf(" [%s]", defaultAnswer)
	}
	fmt.Printf(": ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func CliAskRequired(question string, defaultAnswer string) string {
	for {
		answer := CliAsk(question, defaultAnswer)
		if answer != "" {
			return answer
		} else if defaultAnswer != "" {
			return defaultAnswer
		}
	}
}

func CliAskPassword(question string) string {
	fmt.Printf("%s: ", question)
	password, err := terminal.ReadPassword(0)
	if err != nil {
		panic(errors.Wrapf(err, "failed to read password"))
	}
	fmt.Println()
	return string(password)
}

func CliConfirm(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	answer := ""
	for answer != "y" && answer != "n" {
		fmt.Print(question + " [y/n] ")
		input, _ := reader.ReadByte()
		answer = strings.ToLower(string(input))
	}
	return answer == "y"
}
