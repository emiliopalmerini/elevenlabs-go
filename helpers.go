package elevenlabs

// Bool returns a pointer to v.
func Bool(v bool) *bool { return &v }

// Float64 returns a pointer to v.
func Float64(v float64) *float64 { return &v }

// Int returns a pointer to v.
func Int(v int) *int { return &v }

// String returns a pointer to v.
func String(v string) *string { return &v }
