package wwgraphql

import (
	"context"
	"github.com/99designs/gqlgen/graphql"
	"github.com/weavingwebs/wwgo"
)

// GqlHasFields will return true if any of the target fields exist.
func GqlHasFields(fields []graphql.CollectedField, targetFields ...string) bool {
	for _, f := range fields {
		if wwgo.ArrayIncludesStr(targetFields, f.Name) {
			return true
		}
	}
	return false
}

// GqlGetSubFields will return the target field if found.
func GqlGetSubFields(ctx context.Context, fields []graphql.CollectedField, targetField string) ([]graphql.CollectedField, bool) {
	for _, f := range fields {
		if f.Name == targetField {
			return graphql.CollectFields(graphql.GetOperationContext(ctx), f.Selections, nil), true
		}
	}
	return nil, false
}

// GqlGetNestedSubFields will return the nested target field if found.
func GqlGetNestedSubFields(ctx context.Context, fields []graphql.CollectedField, targetFields ...string) ([]graphql.CollectedField, bool) {
	currentFields := fields
	for _, targetField := range targetFields {
		subFields, ok := GqlGetSubFields(ctx, currentFields, targetField)
		if !ok {
			return nil, false
		}
		currentFields = subFields
	}
	return currentFields, true
}
