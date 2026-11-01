package i18n

import "fmt"

// keyErr is an error whose text is translated when it is read, so sentinel
// errors (compared with errors.Is) can still be shown in the user's language.
type keyErr struct {
	key  string
	args []any
}

func (e *keyErr) Error() string { return T(e.key, e.args...) }

// Err returns a translated error. Two calls give two different errors; keep
// one in a package variable to use it as a sentinel with errors.Is.
func Err(key string, args ...any) error { return &keyErr{key: key, args: args} }

// Errorf is fmt.Errorf with a translated format, so %w keeps working:
//
//	i18n.Errorf("provider.http", status, body)
func Errorf(key string, args ...any) error { return fmt.Errorf(T(key), args...) }
