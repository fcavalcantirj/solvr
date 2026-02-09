package models

import (
	"reflect"
	"testing"
)

// TestResponseModelHasNoDeletedAt is a regression test ensuring the responses
// table/model doesn't support soft-delete. The responses table has no deleted_at
// column (see migration 000006). If someone adds DeletedAt to the Response model,
// they MUST also add a migration to create the deleted_at column on the responses table.
// Without this, queries like "WHERE deleted_at IS NULL" will cause SQL errors.
func TestResponseModelHasNoDeletedAt(t *testing.T) {
	responseType := reflect.TypeOf(Response{})
	_, found := responseType.FieldByName("DeletedAt")
	if found {
		t.Fatal("Response model has DeletedAt field — ensure the responses table also has a " +
			"deleted_at column (via migration) before using deleted_at IS NULL in queries. " +
			"See migration 000006_create_responses.up.sql")
	}
}

// TestSoftDeleteAwareness verifies which models support soft-delete.
// This prevents bugs where queries use deleted_at on tables that don't have it.
func TestSoftDeleteAwareness(t *testing.T) {
	// Models WITH soft-delete (have DeletedAt field, table has deleted_at column)
	softDeleteModels := map[string]any{
		"Answer":   Answer{},
		"Approach": Approach{},
	}
	for name, model := range softDeleteModels {
		rt := reflect.TypeOf(model)
		_, found := rt.FieldByName("DeletedAt")
		if !found {
			t.Errorf("%s should have DeletedAt field for soft-delete support", name)
		}
	}

	// Models WITHOUT soft-delete (no DeletedAt field, table has no deleted_at column)
	noSoftDeleteModels := map[string]any{
		"Response": Response{},
	}
	for name, model := range noSoftDeleteModels {
		rt := reflect.TypeOf(model)
		_, found := rt.FieldByName("DeletedAt")
		if found {
			t.Errorf("%s should NOT have DeletedAt — table has no deleted_at column. "+
				"Add a migration first if soft-delete is needed.", name)
		}
	}
}
