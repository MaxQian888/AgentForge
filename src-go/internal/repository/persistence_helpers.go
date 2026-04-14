package repository

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableStringUpdate(value string) any {
	if value == "" {
		return nil
	}
	return value
}
