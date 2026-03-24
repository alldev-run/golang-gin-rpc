package bootstrap

import (
	"context"

	internalbootstrap "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
)

type Bootstrap = internalbootstrap.Bootstrap

type FrameworkOptions = internalbootstrap.FrameworkOptions

type APIGatewayServiceOptions = internalbootstrap.APIGatewayServiceOptions

func NewBootstrap(configPath string) (*Bootstrap, error) {
	return internalbootstrap.NewBootstrap(configPath)
}

func New(configPath string) (*Bootstrap, error) {
	return internalbootstrap.NewBootstrap(configPath)
}

func DefaultFrameworkOptions() FrameworkOptions {
	return internalbootstrap.DefaultFrameworkOptions()
}

func StartFramework(ctx context.Context, boot *Bootstrap, options FrameworkOptions) error {
	if boot == nil {
		return nil
	}
	return boot.StartFramework(ctx, options)
}
