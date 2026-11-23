package template

import (
	"maps"
	"slices"
	"strings"

	"legnaris/backend/shared/maputils"
)

const (
	AggregateSum   = "sum"
	AggregateCount = "count"
)

type AggregateSchema struct {
	List  string `bson:"list" json:"list"`
	Field string `bson:"field" json:"field"`
	Op    string `bson:"op" json:"op"`
}

func (a *AggregateSchema) Clone() *AggregateSchema {
	if a == nil {
		return nil
	}
	copied := *a
	return &copied
}

func validateAggregate(path string, schema *FieldSchema, lists map[string]ListSchema) error {
	aggregate := schema.Aggregate
	if aggregate == nil {
		return nil
	}
	if schema.Formula != "" {
		return ErrInvalidAggregate(path, "an aggregate leaf cannot declare a formula")
	}
	if schema.IsEditable {
		return ErrInvalidAggregate(path, "an aggregate must be derived (is_editable false)")
	}
	if schema.Type != fieldTypeNumber {
		return ErrInvalidAggregate(path, "an aggregate must be a number leaf")
	}
	if len(schema.DependsOn) > 0 {
		return ErrInvalidAggregate(path, "an aggregate declares its dependency through list and field")
	}
	if aggregate.Op != AggregateSum && aggregate.Op != AggregateCount {
		return ErrInvalidAggregate(path, "aggregate op must be sum or count")
	}

	list, ok := lists[aggregate.List]
	if !ok {
		return ErrInvalidAggregate(path, "aggregate list '"+aggregate.List+"' does not exist")
	}
	if aggregate.Field == ListPositionField {
		return ErrInvalidAggregate(path, "aggregate field is the reserved position")
	}
	node, ok := list.Shape[aggregate.Field]
	if !ok || !node.IsLeaf() || node.Schema == nil {
		return ErrInvalidAggregate(path, "aggregate field '"+aggregate.Field+"' is not in the list shape")
	}
	if node.Schema.Type != fieldTypeNumber {
		return ErrInvalidAggregate(path, "aggregate field '"+aggregate.Field+"' must be a number")
	}
	return nil
}

func (d *DAG) buildAggregates(schemas []*FieldSchema, paths []Path, lists map[string]ListSchema) error {
	for i, schema := range schemas {
		path := paths[i].String()
		if err := validateAggregate(path, schema, lists); err != nil {
			return err
		}
		if schema.Aggregate == nil {
			continue
		}
		d.aggregates[path] = *schema.Aggregate.Clone()
		d.aggregatesByList[schema.Aggregate.List] = append(
			d.aggregatesByList[schema.Aggregate.List], path,
		)
		recordKey := schema.Aggregate.List + "." + schema.Aggregate.Field
		d.adjacencyList[recordKey] = append(d.adjacencyList[recordKey], path)
	}
	return nil
}

func (d *DAG) aggregateValue(aggregate AggregateSchema, sheetData map[string]any) float64 {
	stored, found := maputils.Get(sheetData, strings.Split(aggregate.List, "."))
	if !found {
		return 0
	}
	records, ok := stored.(map[string]any)
	if !ok {
		return 0
	}

	total := 0.0
	for _, recordID := range slices.Sorted(maps.Keys(records)) {
		record, ok := records[recordID].(map[string]any)
		if !ok {
			continue
		}
		value, ok := maputils.ToFloat64(record[aggregate.Field])
		if !ok || value == 0 {
			continue
		}
		switch aggregate.Op {
		case AggregateSum:
			total += value
		case AggregateCount:
			total++
		}
	}
	return total
}
