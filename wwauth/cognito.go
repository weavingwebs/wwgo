package wwauth

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"os"
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
) *CognitoAuth {
	userPoolId := os.Getenv("AUTH_COGNITO_POOL_ID")
	if userPoolId == "" {
		panic("AUTH_COGNITO_POOL_ID is not set")
	}
	clientId := os.Getenv("AUTH_AUD")
	if clientId == "" {
		panic("AUTH_AUD is not set")
	}

	idp := cognitoidentityprovider.NewFromConfig(awsConfig)
	return &CognitoAuth{
		awsConfig:  awsConfig,
		idp:        idp,
		log:        log,
		userPoolId: userPoolId,
		clientId:   clientId,
	}
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

func (c *CognitoAuth) AdminResetPassword(ctx context.Context, id string) (string, error) {
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
