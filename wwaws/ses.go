package wwaws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/weavingwebs/wwgo"
	"os"
	"strings"
)

var DefaultSafeEmailDomains = []string{"weavingwebs.co.uk"}

type SesMailer struct {
	log              zerolog.Logger
	sesClient        *ses.Client
	fromAddress      string
	SafeEmailDomains []string
}

type Email struct {
	To       []string
	Subject  string
	HtmlBody string
}

func NewSesMailer(
	log zerolog.Logger,
	awsConfig aws.Config,
	fromAddress string,
	safeEmailDomains []string,
) *SesMailer {
	sesClient := ses.NewFromConfig(awsConfig)

	return &SesMailer{
		log:              log,
		sesClient:        sesClient,
		fromAddress:      fromAddress,
		SafeEmailDomains: safeEmailDomains,
	}
}

func NewSesMailerFromEnv(
	log zerolog.Logger,
	awsConfig aws.Config,
) *SesMailer {
	fromAddress := os.Getenv("MAIL_FROM_ADDRESS")
	if fromAddress == "" {
		panic("MAIL_FROM_ADDRESS is not set")
	}
	var safeEmailDomains []string
	if os.Getenv("EMAIL_ANY_ADDRESS") != "1" {
		safeEmailDomains = DefaultSafeEmailDomains
	}

	return NewSesMailer(log, awsConfig, fromAddress, safeEmailDomains)
}

func (s *SesMailer) Send(ctx context.Context, email Email) error {
	to := s.FilterUnsafeEmailsAndWarn(email.To)
	if len(to) == 0 {
		s.log.Warn().Msgf("No email was sent")
		return nil
	}

	_, err := s.sesClient.SendEmail(ctx, &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: to,
		},
		Message: &types.Message{
			Body: &types.Body{
				Html: &types.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(email.HtmlBody),
				},
			},
			Subject: &types.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String(email.Subject),
			},
		},
		Source: aws.String(s.fromAddress),
	})

	if err != nil {
		return errors.Wrapf(err, "failed to send email")
	}
	return nil
}

func (s *SesMailer) SendRaw(ctx context.Context, destinations []string, body []byte) error {
	destinations = s.FilterUnsafeEmailsAndWarn(destinations)
	if len(destinations) == 0 {
		s.log.Warn().Msgf("No email was sent")
		return nil
	}

	input := &ses.SendRawEmailInput{
		RawMessage: &types.RawMessage{
			Data: body,
		},
		Destinations: destinations,
		Source:       aws.String(s.fromAddress),
	}

	_, err := s.sesClient.SendRawEmail(ctx, input)
	if err != nil {
		return errors.Wrapf(err, "failed to send raw email")
	}
	return nil
}

func (s *SesMailer) CheckSendToAddress(email string) bool {
	if len(s.SafeEmailDomains) == 0 {
		return true
	}

	emailParts := strings.SplitN(email, "@", 2)
	if len(emailParts) < 2 {
		s.log.Err(errors.Errorf("invalid email %s", email)).Send()
		return false
	}
	return wwgo.ArrayIncludesStr(s.SafeEmailDomains, emailParts[1])
}

func (s *SesMailer) FilterUnsafeEmailsAndWarn(emails []string) []string {
	if len(s.SafeEmailDomains) == 0 {
		return emails
	}
	originalEmails := emails
	emails = wwgo.ArrayFilterFnStr(emails, s.CheckSendToAddress)
	unsafeEmails := wwgo.ArrayDiffStr(originalEmails, emails)
	if len(unsafeEmails) != 0 {
		s.log.Warn().Msgf("Refusing to send email to %s", strings.Join(unsafeEmails, ", "))
	}
	return emails
}
