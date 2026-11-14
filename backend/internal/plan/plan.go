// Package plan holds the code-defined subscription plans and the per-kind Sheet
// allowances they grant. It has no dependency on the sheet or user packages, so
// both can read it without an import cycle.
package plan

// SheetKind names what a Sheet represents. It is immutable once a Sheet is
// created and decides which per-plan allowance the Sheet counts against.
type SheetKind string

const (
	KindCharacter SheetKind = "character"
	KindNPC       SheetKind = "npc"
	KindMonster   SheetKind = "monster"
)

var sheetKinds = []SheetKind{KindCharacter, KindNPC, KindMonster}

// SheetKinds returns every valid kind in a stable order. The slice is a copy,
// so callers cannot alter the registry.
func SheetKinds() []SheetKind {
	return append([]SheetKind(nil), sheetKinds...)
}

// ParseSheetKind reports whether raw names a valid kind. It is exact: no
// trimming and no case folding, so a malformed value is rejected rather than
// silently normalized.
func ParseSheetKind(raw string) (SheetKind, bool) {
	kind := SheetKind(raw)
	for _, known := range sheetKinds {
		if kind == known {
			return kind, true
		}
	}
	return "", false
}

// Plan is one subscription tier and the number of Sheets it allows per kind.
// The limits gate creation only; a downgrade never locks existing Sheets.
type Plan struct {
	ID          string
	sheetLimits map[SheetKind]int
}

// SheetLimit returns the number of Sheets of kind the plan allows. An unknown
// kind has no allowance, so it reads as zero.
func (p Plan) SheetLimit(kind SheetKind) int {
	return p.sheetLimits[kind]
}

// IDFree and IDPaid are the stored plan identifiers.
const (
	IDFree = "free"
	IDPaid = "paid"
)

// Free is the default plan: every account without a stored plan is on it.
var Free = Plan{
	ID: IDFree,
	sheetLimits: map[SheetKind]int{
		KindCharacter: 1,
		KindNPC:       2,
		KindMonster:   2,
	},
}

// Paid is the single paid plan shipped with F1a.
var Paid = Plan{
	ID: IDPaid,
	sheetLimits: map[SheetKind]int{
		KindCharacter: 10,
		KindNPC:       50,
		KindMonster:   50,
	},
}

// Lookup resolves a stored plan identifier. A missing or unknown identifier
// resolves to Free, the smallest allowance, so an unexpected value never grants
// more than the default.
func Lookup(id string) Plan {
	if id == IDPaid {
		return Paid
	}
	return Free
}
