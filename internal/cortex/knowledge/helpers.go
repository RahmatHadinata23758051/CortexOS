package knowledge

func projectIDPtr(s string) *ProjectID {
	id := ProjectID(s)
	return &id
}

func lifecycleActivePtr() *LifecycleState {
	state := LifecycleActive
	return &state
}
