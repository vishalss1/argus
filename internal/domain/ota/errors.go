package ota

import "errors"

var (
	ErrFirmwareNotFound   = errors.New("firmware artifact not found")
	ErrDeploymentNotFound = errors.New("ota deployment not found")
)
