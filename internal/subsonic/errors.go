package subsonic

// Subsonic error codes (see
// https://opensubsonic.netlify.app/docs/responses/error/). Every failure
// envelope written by this package must carry one of these codes.
const (
	ErrGeneric           = 0
	ErrMissingParam      = 10
	ErrClientVersionOld  = 20
	ErrServerVersionOld  = 30
	ErrBadCredentials    = 40
	ErrLDAPTokenAuth     = 41
	ErrAuthNotSupported  = 42
	ErrConflictingAuth   = 43
	ErrInvalidAPIKey     = 44
	ErrNotAuthorized     = 50
	ErrTrialExpired      = 60
	ErrNotFound          = 70
)
