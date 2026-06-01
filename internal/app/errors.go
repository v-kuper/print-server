package app

func optionalWarnings(values ...string) []string {
	var warnings []string
	for _, value := range values {
		if value != "" {
			warnings = append(warnings, value)
		}
	}
	return warnings
}

func buildError(status int, err error) error {
	return BuildError{Status: status, Err: err}
}
