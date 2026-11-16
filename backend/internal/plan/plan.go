package plan

type SheetKind string

const (
	KindCharacter SheetKind = "character"
	KindNPC       SheetKind = "npc"
	KindMonster   SheetKind = "monster"
)

var sheetKinds = []SheetKind{KindCharacter, KindNPC, KindMonster}

func SheetKinds() []SheetKind {
	return append([]SheetKind(nil), sheetKinds...)
}

func ParseSheetKind(raw string) (SheetKind, bool) {
	kind := SheetKind(raw)
	for _, known := range sheetKinds {
		if kind == known {
			return kind, true
		}
	}
	return "", false
}

type Plan struct {
	ID          string
	sheetLimits map[SheetKind]int
}

func (p Plan) SheetLimit(kind SheetKind) int {
	return p.sheetLimits[kind]
}

const (
	IDFree = "free"
	IDPaid = "paid"
)

var Free = Plan{
	ID: IDFree,
	sheetLimits: map[SheetKind]int{
		KindCharacter: 1,
		KindNPC:       2,
		KindMonster:   2,
	},
}

var Paid = Plan{
	ID: IDPaid,
	sheetLimits: map[SheetKind]int{
		KindCharacter: 10,
		KindNPC:       50,
		KindMonster:   50,
	},
}

func Lookup(id string) Plan {
	if id == IDPaid {
		return Paid
	}
	return Free
}
