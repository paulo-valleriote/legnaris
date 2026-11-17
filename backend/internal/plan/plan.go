package plan

type SheetKind string

const (
	KindCharacter SheetKind = "character"
	KindNPC       SheetKind = "npc"
	KindEnemy     SheetKind = "enemy"
)

var sheetKinds = []SheetKind{KindCharacter, KindNPC, KindEnemy}

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
	ID             string
	sheetLimits    map[SheetKind]int
	ownedCampaigns int
}

func (p Plan) SheetLimit(kind SheetKind) int {
	return p.sheetLimits[kind]
}

func (p Plan) OwnedCampaignLimit() int {
	return p.ownedCampaigns
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
		KindEnemy:     2,
	},
	ownedCampaigns: 1,
}

var Paid = Plan{
	ID: IDPaid,
	sheetLimits: map[SheetKind]int{
		KindCharacter: 10,
		KindNPC:       50,
		KindEnemy:     50,
	},
	ownedCampaigns: 5,
}

func Lookup(id string) Plan {
	if id == IDPaid {
		return Paid
	}
	return Free
}
