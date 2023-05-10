package wwauth

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"os"
	"strings"
	"time"
)

type CognitoAuth struct {
	awsConfig  aws.Config
	idp        *cognitoidentityprovider.Client
	log        zerolog.Logger
	userPoolId string
	clientId   string
}

// CognitoAuthPublicSettings should precisely represent the 'UserPoolConfig'
// interface in ww-cognito-react.
type CognitoAuthPublicSettings struct {
	UserPoolId string `json:"UserPoolId"`
	ClientId   string `json:"ClientId"`
	Region     string `json:"Region"`
}

func NewCognitoAuth(
	log zerolog.Logger,
	awsConfig aws.Config,
	userPoolId string,
	clientId string,
) *CognitoAuth {
	idp := cognitoidentityprovider.NewFromConfig(awsConfig)
	return &CognitoAuth{
		awsConfig:  awsConfig,
		idp:        idp,
		log:        log,
		userPoolId: userPoolId,
		clientId:   clientId,
	}
}

func NewCognitoAuthFromEnv(
	log zerolog.Logger,
	awsConfig aws.Config,
) *CognitoAuth {
	userPoolId := os.Getenv("AUTH_COGNITO_POOL_ID")
	if userPoolId == "" {
		panic("AUTH_COGNITO_POOL_ID is not set")
	}
	clientId := os.Getenv("AUTH_AUD")
	if clientId == "" {
		panic("AUTH_AUD is not set")
	}

	return NewCognitoAuth(log, awsConfig, userPoolId, clientId)
}

func (c *CognitoAuth) PublicSettings() CognitoAuthPublicSettings {
	return CognitoAuthPublicSettings{
		UserPoolId: c.userPoolId,
		ClientId:   c.clientId,
		Region:     c.awsConfig.Region,
	}
}

func (c *CognitoAuth) Idp() *cognitoidentityprovider.Client {
	return c.idp
}

func (c *CognitoAuth) UserPoolId() string {
	return c.userPoolId
}

type AdminCreateUserOpt struct {
	Attributes        []types.AttributeType
	TemporaryPassword string
	SuppressEmail     bool
}

func (c *CognitoAuth) AdminCreateUser(ctx context.Context, email string, opt AdminCreateUserOpt) (*types.UserType, error) {
	if opt.Attributes == nil {
		opt.Attributes = []types.AttributeType{}
	}
	opt.Attributes = append(opt.Attributes, types.AttributeType{
		Name:  aws.String("email"),
		Value: aws.String(email),
	})

	if opt.TemporaryPassword == "" {
		opt.TemporaryPassword = RandomHumanPassword()
	}

	input := &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId:        aws.String(c.userPoolId),
		Username:          aws.String(email),
		TemporaryPassword: aws.String(opt.TemporaryPassword),
		UserAttributes:    opt.Attributes,
	}

	if opt.SuppressEmail {
		input.MessageAction = types.MessageActionTypeSuppress
	}

	res, err := c.idp.AdminCreateUser(ctx, input)
	if err != nil {
		return nil, errors.Wrap(err, "error creating user")
	}
	return res.User, err
}

func (c *CognitoAuth) AdminGetUser(ctx context.Context, id string) (*cognitoidentityprovider.AdminGetUserOutput, error) {
	res, err := c.idp.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(c.userPoolId),
		Username:   aws.String(id),
	})
	if err != nil {
		var userNotFoundErr *types.UserNotFoundException
		if errors.As(err, &userNotFoundErr) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to get user %s", id)
	}
	return res, nil
}

func (c *CognitoAuth) AdminResendTemporaryPassword(ctx context.Context, email string) error {
	input := &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId:        aws.String(c.userPoolId),
		Username:          aws.String(email),
		TemporaryPassword: aws.String(RandomHumanPassword()),
		UserAttributes:    []types.AttributeType{},
		MessageAction:     types.MessageActionTypeResend,
	}
	_, err := c.idp.AdminCreateUser(ctx, input)
	if err != nil {
		return errors.Wrap(err, "error resending email")
	}
	return nil
}

// AdminSetTemporaryPassword sets the user's password to a temporary 'human'
// password. The user will need to set a password when they login.
func (c *CognitoAuth) AdminSetTemporaryPassword(ctx context.Context, id string) (string, error) {
	tmpPassword := RandomHumanPassword()
	_, err := c.idp.AdminSetUserPassword(ctx, &cognitoidentityprovider.AdminSetUserPasswordInput{
		UserPoolId: aws.String(c.userPoolId),
		Username:   aws.String(id),
		Permanent:  false,
		Password:   aws.String(tmpPassword),
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to reset user %s", id)
	}
	return tmpPassword, nil
}

// ListUsers with pagination handling.
// This is always important as cognito will sometimes return an empty page with a token.
func (c *CognitoAuth) ListUsers(ctx context.Context, input *cognitoidentityprovider.ListUsersInput) ([]types.UserType, error) {
	doListUsers := func(paginationToken *string) (*cognitoidentityprovider.ListUsersOutput, error) {
		input.PaginationToken = paginationToken
		if input.Limit == nil {
			input.Limit = aws.Int32(60)
		}

		var err error
		i := 0
		for i < 5 {
			i++
			res, err := c.idp.ListUsers(ctx, input)
			if err != nil {
				var tooManyRequestsErr *types.TooManyRequestsException
				if errors.As(err, &tooManyRequestsErr) {
					time.Sleep(time.Second)
					continue
				}
				return nil, errors.Wrap(err, "failed to list users")
			}
			return res, nil
		}
		return nil, errors.Wrapf(err, "failed to list users (retry %d)", i)
	}

	users := make([]types.UserType, 0)
	var paginationToken *string
	for {
		res, err := doListUsers(paginationToken)
		if err != nil {
			return nil, err
		}
		users = append(users, res.Users...)
		if res.PaginationToken == nil {
			break
		}
		paginationToken = res.PaginationToken
	}

	return users, nil
}

// AwsFilter helps format and attempts to escape filter expressions.
func AwsFilter(field string, op string, value string) *string {
	value = strings.ReplaceAll(strings.TrimSpace(value), `"`, `\"`)
	res := fmt.Sprintf(`%s %s "%s"`, field, op, value)
	return &res
}

func UserTypeFromAdminGetUserResult(res *cognitoidentityprovider.AdminGetUserOutput) types.UserType {
	return types.UserType{
		Attributes:           res.UserAttributes,
		Enabled:              res.Enabled,
		MFAOptions:           res.MFAOptions,
		UserCreateDate:       res.UserCreateDate,
		UserLastModifiedDate: res.UserLastModifiedDate,
		UserStatus:           res.UserStatus,
		Username:             res.Username,
	}
}
