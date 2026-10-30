package gabric

// Must panics if err is not nil. It is a helper for calling TaskErr tasks and
// error-returning operations from panic-style task code.
func Must(err error) {
	if err != nil {
		panic(err)
	}
}
