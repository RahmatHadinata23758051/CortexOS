package sqlite

import "database/sql"

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value *string) any {
	if value == nil {
		return nil
	}
	return sql.Named("finishedAt", *value)
}
