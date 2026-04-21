package subsonic

// ptr returns a pointer to v. Handy for the many optional fields in
// the generated Subsonic types, which use `*T` for nullable values.
func ptr[T any](v T) *T { return &v }
