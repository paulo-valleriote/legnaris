package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLayout_RecordRows(t *testing.T) {
	tmpl := rowIdentityTemplate()

	tests := []struct {
		name    string
		pinned  []string
		wantErr error
	}{
		{
			name:   "Happy path: a record row pins by its list path and id",
			pinned: []string{"inventory.items." + rowIdentityRecordID},
		},
		{
			name:   "Happy path: static and record rows pin together",
			pinned: []string{"stats.strength", "inventory.items." + rowIdentityRecordID},
		},
		{
			name:   "Edge: a well-formed id needs no matching Sheet Record",
			pinned: []string{"inventory.items.rffffffffffffffffffffffffffffffff"},
		},
		{
			name:    "Error: a malformed record id is not a row",
			pinned:  []string{"inventory.items.rbad"},
			wantErr: &UnknownLayoutRowError{Key: "inventory.items.rbad"},
		},
		{
			name:    "Error: an unknown list is not a row",
			pinned:  []string{"unknown.items." + rowIdentityRecordID},
			wantErr: &UnknownLayoutRowError{Key: "unknown.items." + rowIdentityRecordID},
		},
		{
			name:    "Error: the list path alone is not a row",
			pinned:  []string{"inventory.items"},
			wantErr: &UnknownLayoutRowError{Key: "inventory.items"},
		},
		{
			name:    "Error: a record field path is not a row",
			pinned:  []string{"inventory.items." + rowIdentityRecordID + ".quantity"},
			wantErr: &UnknownLayoutRowError{Key: "inventory.items." + rowIdentityRecordID + ".quantity"},
		},
		{
			name:    "Error: a duplicate record pin is refused",
			pinned:  []string{"inventory.items." + rowIdentityRecordID, "inventory.items." + rowIdentityRecordID},
			wantErr: &DuplicateLayoutKeyError{List: layoutListPinned, Key: "inventory.items." + rowIdentityRecordID},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLayout(tmpl, nil, tc.pinned)
			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func TestValidateLayout_ListOnlySection(t *testing.T) {
	tmpl := Template{Fields: FieldGroup{
		"items": {List: &ListSchema{
			Label:    "Items",
			Capacity: 1,
			Shape:    FieldGroup{"quantity": rowNumberLeaf()},
		}},
	}}

	require.NoError(t, ValidateLayout(tmpl, []string{"items"}, nil))
	assert.Equal(t, &UnknownLayoutSectionError{Key: "other"}, ValidateLayout(tmpl, []string{"other"}, nil))
}
