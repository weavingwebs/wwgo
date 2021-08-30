package wwauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	"net/url"
	"strconv"
	"time"
)

type UrlSigner struct {
	privateKey []byte
}

func NewUrlSigner(privateKey []byte) *UrlSigner {
	return &UrlSigner{
		privateKey: privateKey,
	}
}

func NewUrlSignerRandom() *UrlSigner {
	return NewUrlSigner([]byte(RandomAlphanumeric(256)))
}

func (us *UrlSigner) SignUrl(requestUrl *url.URL, expirySeconds int) *url.URL {
	return us.SignUrlForTime(requestUrl, expirySeconds, time.Now())
}

func (us *UrlSigner) SignUrlForTime(requestUrl *url.URL, expirySeconds int, signedAt time.Time) *url.URL {
	params := requestUrl.Query()
	params.Set("X-Signed-At", fmt.Sprintf("%d", signedAt.Unix()))
	params.Set("X-Expires", fmt.Sprintf("%d", expirySeconds))

	tmpUrl, _ := url.Parse(requestUrl.RequestURI())
	tmpUrl.RawQuery = params.Encode() // NOTE: This also orders the params.

	mac := hmac.New(sha256.New, us.privateKey)
	mac.Write([]byte(tmpUrl.RequestURI()))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	params.Set("X-Signature", sig)

	// Bring back hostname etc.
	signedUrl, _ := url.Parse(requestUrl.String())
	signedUrl.RawQuery = params.Encode()
	return signedUrl
}

func (us UrlSigner) VerifyUrl(rawUrl string) (bool, error) {
	requestUrl, err := url.ParseRequestURI(rawUrl)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse url %s", rawUrl)
	}
	params := requestUrl.Query()

	signedAt, err := strconv.ParseInt(params.Get("X-Signed-At"), 10, 64)
	if err != nil {
		return false, nil
	}

	expires, err := strconv.Atoi(params.Get("X-Expires"))
	if err != nil || expires < 1 {
		return false, nil
	}

	expiryTime := time.Unix(signedAt, 0).Add(time.Second * time.Duration(expires))
	if expiryTime.Before(time.Now()) {
		return false, nil
	}

	signature := params.Get("X-Signature")
	if signature == "" {
		return false, nil
	}
	params.Del("X-Signature")

	originalUrl, _ := url.Parse(requestUrl.RequestURI())
	originalUrl.RawQuery = params.Encode() // NOTE: This also orders the params.
	expectedUrl := us.SignUrlForTime(originalUrl, expires, time.Unix(signedAt, 0))

	expectedSig := expectedUrl.Query().Get("X-Signature")
	return hmac.Equal([]byte(signature), []byte(expectedSig)), nil
}
