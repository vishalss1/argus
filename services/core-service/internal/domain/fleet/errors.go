package fleet

import "errors"

var ErrFleetNotFound = errors.New("fleet not found")
var ErrFleetNameEmpty = errors.New("fleet name is required")
var ErrNodeCountInvalid = errors.New("node count must be between 1 and 500")
var ErrHardwareTypeEmpty = errors.New("hardware type is required")
