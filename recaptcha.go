package wwgo

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/weavingwebs/wwgo/wwhttp"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
)

type ReCaptchaV2Response struct {
	Success     bool     `json:"success"`
	ChallengeTs string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

type ReCaptchaV3Response struct {
	Success     bool     `json:"success"`
	Score       float32  `json:"score"`
	Action      string   `json:"action"`
	ChallengeTs string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

type ReCaptcha struct {
	Log         zerolog.Logger
	SecretKeyV2 string
	SiteKeyV2   string
	SecretKeyV3 string
	SiteKeyV3   string
	MinScoreV3  float32
}

type ReCaptchaPublicSettings struct {
	SiteKeyV2 string `json:"siteKeyV2"`
	SiteKeyV3 string `json:"siteKeyV3"`
}

func NewReCaptchaFromEnv(log zerolog.Logger) ReCaptcha {
	res := ReCaptcha{
		Log:         log,
		SecretKeyV2: os.Getenv("RECAPTCHA_V2_SECRET_KEY"),
		SiteKeyV2:   os.Getenv("RECAPTCHA_V2_SITE_KEY"),
		SecretKeyV3: os.Getenv("RECAPTCHA_V3_SECRET_KEY"),
		SiteKeyV3:   os.Getenv("RECAPTCHA_V3_SITE_KEY"),
		MinScoreV3:  0.9,
	}

	if res.SecretKeyV2 == "" {
		panic("RECAPTCHA_V2_SECRET_KEY is not set")
	}
	if res.SiteKeyV2 == "" {
		panic("RECAPTCHA_V2_SITE_KEY is not set")
	}
	if res.SecretKeyV3 == "" {
		panic("RECAPTCHA_V3_SECRET_KEY is not set")
	}
	if res.SiteKeyV3 == "" {
		panic("RECAPTCHA_V3_SITE_KEY is not set")
	}

	score := os.Getenv("RECAPTCHA_V3_SCORE")
	if score != "" {
		f, err := strconv.ParseFloat(score, 32)
		if err != nil {
			panic(errors.Wrapf(err, "failed to parse RECAPTCHA_V3_SCORE"))
		}
		res.MinScoreV3 = float32(f)
	}
	return res
}

func (r ReCaptcha) PublicSettings() ReCaptchaPublicSettings {
	return ReCaptchaPublicSettings{
		SiteKeyV2: r.SiteKeyV2,
		SiteKeyV3: r.SiteKeyV3,
	}
}

func (r ReCaptcha) ReCaptchaV3Verify(ctx context.Context, token string) (*ReCaptchaV3Response, error) {
	secret := r.SecretKeyV3
	if secret != "" {
		return nil, errors.Errorf("reCAPTCHA V3 key is not set")
	}
	resp, err := reCaptchaVerifyFromContext(ctx, secret, token)
	var result ReCaptchaV3Response
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse reCAPTCHA V3 verification")
	}
	r.Log.Trace().Msgf("reCAPTCHA V3 verify response: %s", resp)
	return &result, nil
}

func (r ReCaptcha) ReCaptchaV2Verify(ctx context.Context, token string) (*ReCaptchaV2Response, error) {
	secret := r.SecretKeyV2
	if secret != "" {
		return nil, errors.Errorf("reCAPTCHA V2 key is not set")
	}
	resp, err := reCaptchaVerifyFromContext(ctx, secret, token)
	var result ReCaptchaV2Response
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse reCAPTCHA V2 verification")
	}
	r.Log.Trace().Msgf("reCAPTCHA V2 verify response: %s", resp)
	return &result, nil
}

type ReCaptchaTokens struct {
	ReCaptchaV3token *string
	ReCaptchaV2token *string
}

func (r ReCaptcha) VerifyReCaptchaTokensFromContext(ctx context.Context, action string, tokens ReCaptchaTokens) error {
	if tokens.ReCaptchaV2token != nil {
		result, err := r.ReCaptchaV2Verify(ctx, *tokens.ReCaptchaV2token)
		if err != nil {
			return err
		}
		if !result.Success {
			return NewClientError("RECAPTCHA_V2_VERIFICATION_EXCEPTION", "Anti-bot verification failed, please try again", nil)
		}
		return nil
	} else if tokens.ReCaptchaV3token != nil {
		result, err := r.ReCaptchaV3Verify(ctx, *tokens.ReCaptchaV3token)
		if err != nil {
			return err
		}
		if !result.Success {
			return NewClientError("RECAPTCHA_V3_VERIFICATION_EXCEPTION", "Invalid reCAPTCHA V3 token", nil)
		}
		r.Log.Info().Msgf("reCAPTCHA score: %f", result.Score)
		if result.Action != action {
			return NewClientError("RECAPTCHA_V3_ACTION_EXCEPTION", "Invalid reCAPTCHA V3 action", nil)
		}
		if result.Score < r.MinScoreV3 {
			return NewClientError("RECAPTCHA_V3_SCORE_BELOW_THRESHOLD_EXCEPTION", "reCAPTCHA V3 score is below threshold, V2 required", nil)
		}
		return nil
	}
	return NewClientError("RECAPTCHA_MISSING_TOKEN_EXCEPTION", "reCAPTCHA V2 or reCAPTCHA V3 token required", nil)
}

func reCaptchaVerify(ip net.IP, secret string, token string) ([]byte, error) {
	data := map[string][]string{
		"secret":   {secret},
		"response": {token},
	}
	if len(ip) != 0 && !ip.IsLoopback() {
		data["remoteIp"] = []string{ip.String()}
	}
	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", data)
	if err != nil {
		return []byte{}, errors.Wrap(err, "reCAPTCHA verification request failed")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, errors.Wrap(err, "failed to read reCAPTCHA verification request body")
	}
	return body, nil
}

func reCaptchaVerifyFromContext(ctx context.Context, secret string, token string) ([]byte, error) {
	ip := wwhttp.IpForContext(ctx)
	return reCaptchaVerify(ip, secret, token)
}
