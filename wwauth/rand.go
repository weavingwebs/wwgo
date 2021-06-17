package wwauth

import (
	"crypto/rand"
	"github.com/pkg/errors"
	"math/big"
	"strings"
)

func GenerateRandomString(length int, charset []rune) string {
	res := make([]rune, length)
	max := big.NewInt(int64(len(charset) - 1))
	for i := 0; i < length; i++ {
		randInt, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(errors.Wrapf(err, "failed to generate random int"))
		}
		res[i] = charset[randInt.Int64()]
	}
	return string(res)
}

func RandomHumanPassword() string {
	const vowels = "AEIOU"
	const consonants = "BCDFGHJKLMNPQRSTVWXYZ"
	const numbers = "0123456789"
	return strings.Join([]string{
		strings.ToUpper(GenerateRandomString(1, []rune(consonants))),
		strings.ToLower(GenerateRandomString(1, []rune(vowels))),
		strings.ToLower(GenerateRandomString(1, []rune(consonants))),
		GenerateRandomString(5, []rune(numbers)),
	}, "")
}
