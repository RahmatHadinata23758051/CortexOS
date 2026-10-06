package workspace

import (
	"context"
	"reflect"
	"testing"
)

type compileOnlyStore struct{}

func (compileOnlyStore) Open(context.Context) error { return nil }
func (compileOnlyStore) Close() error               { return nil }
func (compileOnlyStore) SchemaVersion(context.Context) (string, error) {
	return ContractVersion, nil
}

func TestWorkspacePortsAreNarrowAndContextAware(t *testing.T) {
	t.Parallel()

	var _ StateStore = compileOnlyStore{}
	storeType := reflect.TypeOf((*StateStore)(nil)).Elem()
	for i := 0; i < storeType.NumMethod(); i++ {
		method := storeType.Method(i)
		if method.Name == "Close" {
			continue
		}
		if method.Type.NumIn() < 2 || method.Type.In(1) != reflect.TypeOf((*context.Context)(nil)).Elem() {
			t.Fatalf("%s must accept context.Context", method.Name)
		}
	}
}
