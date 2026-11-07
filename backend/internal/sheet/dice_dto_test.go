package sheet_test

import (
	"encoding/json"
	"testing"

	"legnaris/backend/internal/sheet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func text(s string) *string { return &s }

func TestSheetToDTO_Dice(t *testing.T) {
	t.Run("Edge: a sheet that never had dice serializes an empty block, not a missing key", func(t *testing.T) {
		raw, err := json.Marshal(sheet.Sheet{ID: "x"}.ToDTO())
		require.NoError(t, err)

		assert.Contains(t, string(raw), `"dice":{"section":{},"row":{}}`)
	})

	t.Run("Edge: a stored but empty block also serializes empty maps", func(t *testing.T) {
		raw, err := json.Marshal(sheet.Sheet{ID: "x", Dice: &sheet.Dice{}}.ToDTO())
		require.NoError(t, err)

		assert.Contains(t, string(raw), `"dice":{"section":{},"row":{}}`)
	})

	t.Run("Edge: null stays null and is distinct from an absent key", func(t *testing.T) {
		stored := sheet.Sheet{Dice: &sheet.Dice{
			Section: map[string]*string{"stats": nil},
			Row:     map[string]*string{"stats.strength": text("1d20 + @")},
		}}

		raw, err := json.Marshal(stored.ToDTO())
		require.NoError(t, err)

		assert.Contains(t, string(raw), `"dice":{"section":{"stats":null},"row":{"stats.strength":"1d20 + @"}}`)
	})

	t.Run("Edge: the DTO does not alias the stored block in either direction", func(t *testing.T) {
		stored := sheet.Sheet{Dice: &sheet.Dice{
			Section: map[string]*string{"stats": text("1d20")},
			Row:     map[string]*string{"stats.strength": text("1d20 + @")},
		}}

		dto := stored.ToDTO()
		dto.Dice.Section["tampered"] = nil
		*dto.Dice.Row["stats.strength"] = "tampered"
		delete(dto.Dice.Section, "stats")

		assert.Equal(t, map[string]*string{"stats": text("1d20")}, stored.Dice.Section)
		assert.Equal(t, map[string]*string{"stats.strength": text("1d20 + @")}, stored.Dice.Row)
	})
}

func TestDice_IsEmptyCloneAndBlock(t *testing.T) {
	var nilDice *sheet.Dice

	assert.True(t, nilDice.IsEmpty())
	assert.Nil(t, nilDice.Clone())
	assert.True(t, (&sheet.Dice{}).IsEmpty())
	assert.True(t, (&sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{}}).IsEmpty())
	assert.False(t, (&sheet.Dice{Section: map[string]*string{"a": nil}}).IsEmpty(), "a null entry is a choice, not emptiness")
	assert.False(t, (&sheet.Dice{Row: map[string]*string{"a": text("d4")}}).IsEmpty())

	assert.Equal(t, sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{}}, nilDice.Block())
	assert.Equal(t, &sheet.Dice{}, (&sheet.Dice{}).Clone())
}

func TestDice_DecodesNullAsAnExplicitEntry(t *testing.T) {
	var got sheet.Dice

	require.NoError(t, json.Unmarshal([]byte(`{"section":{"stats":null},"row":{}}`), &got))

	value, present := got.Section["stats"]
	assert.True(t, present)
	assert.Nil(t, value)
	_, absent := got.Section["skills"]
	assert.False(t, absent)
}

func TestDice_BSONRoundTripKeepsNullDistinctFromAbsent(t *testing.T) {
	original := sheet.Sheet{ID: "x", Dice: &sheet.Dice{
		Section: map[string]*string{"stats": nil, "skills": text("1d20")},
		Row:     map[string]*string{"stats.strength": nil},
	}}

	raw, err := bson.Marshal(original)
	require.NoError(t, err)
	var got sheet.Sheet
	require.NoError(t, bson.Unmarshal(raw, &got))

	assert.Equal(t, original.Dice, got.Dice)
	value, present := got.Dice.Section["stats"]
	assert.True(t, present)
	assert.Nil(t, value)
	_, absent := got.Dice.Section["config"]
	assert.False(t, absent)
}

func TestDice_BSONOmitsAnEmptyBlockAndReadsLegacySheetsAsNil(t *testing.T) {
	raw, err := bson.Marshal(sheet.Sheet{ID: "x"})
	require.NoError(t, err)

	var asMap bson.M
	require.NoError(t, bson.Unmarshal(raw, &asMap))
	_, stored := asMap["dice"]
	assert.False(t, stored)

	var got sheet.Sheet
	require.NoError(t, bson.Unmarshal(raw, &got))
	assert.Nil(t, got.Dice)

	nullRaw, err := bson.Marshal(bson.M{"_id": "x", "dice": nil})
	require.NoError(t, err)
	var fromNull sheet.Sheet
	require.NoError(t, bson.Unmarshal(nullRaw, &fromNull))
	assert.Nil(t, fromNull.Dice, "a cleared block is stored as null and must read back as nil")
}
