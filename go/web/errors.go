package web

import "toolkit/errors"

const (
	baseErrorCode                 = 2000
	requiredFieldMissingErrorCode = baseErrorCode + 1
)

func init() {
	const errorSpace = "toolkit.web"
	errors.GlobalDictionary.RegisterSpace(errorSpace, "Web utilities", baseErrorCode, baseErrorCode+999)
	errors.GlobalDictionary.RegisterError(errorSpace, requiredFieldMissingErrorCode, "Required field missing")
}
