# Legnaris

A character-sheet manager for tabletop RPGs. Generic templates describe a game system's fields and formulas; a player's sheet fills them in and derived values recalculate on their own.

## Language

**Template**:
A code-defined, read-only description of a game's fields, formulas and default presentation. A Template may target one Game system or none.
_Avoid_: System, ruleset, schema

**Game system**:
A published tabletop RPG with its own rules, such as a specific edition of a fantasy or horror game. A Template that targets a Game system follows that system's rules and uses only openly licensed material from it, with attribution.
_Avoid_: Ruleset, game, edition

**Mechanic family**:
A kind of core dice mechanic shared by several Game systems, such as percentile roll-under. A Template may follow a Mechanic family instead of a Game system only when no open license covers the system its players want.
_Avoid_: Generic system, engine type

**Draft Template**:
A Template that exists but is not yet offered to players: it stays out of the catalog and no Sheet or Campaign can be created from it in production. It becomes an ordinary Template when it is released.
_Avoid_: Hidden, beta, preview, unlisted Template

**Retired Template**:
A Template no longer offered for new Sheets or new Campaigns outside the Campaigns already built on it. Its existing Sheets, Campaigns and links keep working, and a Campaign already fixed to it can still gain Sheets from it.
_Avoid_: Deprecated, legacy, archived Template

**Sheet**:
A character, NPC or enemy built from one Template, holding the values entered and the arrangement chosen. Its Sheet kind says which of the three it is.
_Avoid_: Character, document

**Sheet kind**:
What a Sheet represents: a player's character, an NPC or an enemy. A Template states which kinds it accepts; every kind a Template accepts uses the same Fields and formulas, so the kind changes who uses it and how, never how its values are computed. The kind is set at creation and never changes. An NPC or enemy belongs to the user who made it, exists without any Campaign, and can be linked to every Campaign where its owner is a Game Master; every Campaign sees the same Sheet.
_Avoid_: Sheet type, category

**Plan**:
A user's subscription level. It sets that user's limits, such as how many Sheets of each Sheet kind they may own; every user has one, starting on the free Plan. A Plan belongs to one user and never extends to the other members of their Campaigns.
_Avoid_: Tier, subscription, package

**Campaign**:
A shared space where one or more Game Masters run a game with Players. It fixes one Game system at creation, or one Template when that Template targets no Game system, and accepts only Sheets whose Template falls within it. Membership and role are per Campaign: the same person can be a Game Master in one and a Player in another.
_Avoid_: Group, party, table

**Game Master**:
A Campaign member who runs the game. Shortened to GM. Reads the Players' character Sheets in that Campaign but never edits them, and has no character of their own there.
_Avoid_: Master, DM, narrator

**Owner**:
The one Game Master of a Campaign who created it or received it by transfer. Only the Owner deletes the Campaign, transfers ownership and promotes, demotes or removes GMs. A flag on a GM, not a third role.
_Avoid_: Admin, creator, host

**Invite code**:
The single shareable code (or `/join/<code>` link) that lets someone join a Campaign as a Player. It never expires; any GM can rotate it, which invalidates the previous one.
_Avoid_: Invite link, token, key

**Campaign password**:
An optional secret a GM sets on a Campaign so that joining needs the Invite code and the password. Changing or clearing it removes no member.
_Avoid_: PIN, passphrase

**Deleting**:
The state a Campaign enters when its Owner deletes it, while its Sheets are unlinked in the background. A deleting Campaign is invisible to everyone and is removed once no Sheet is linked to it.
_Avoid_: Archived, soft-deleted, trashed

**Reveal**:
A Game Master making a linked NPC or enemy visible, read-only, to the Players of one Campaign. Per Campaign and per whole Sheet; a new link starts hidden and any Game Master can reveal or hide it.
_Avoid_: Share, publish, show

**Player**:
A Campaign member who plays a character in it, and reads the NPCs and enemies revealed there.
_Avoid_: Member, user

**Field**:
A single named value on a Sheet, or a Record list, either editable by the player or derived from other fields by a formula.
_Avoid_: Stat, property, attribute

**Record list**:
A Field holding any number of Records that the player adds and removes, all sharing the shape the Template gives the list.
_Avoid_: Repeatable, array, table

**Record**:
One entry of a Record list, such as one attack, one item or one spell, made of the Fields the list's shape names; its Derived fields may read its own Fields and the rest of the Sheet.
_Avoid_: Item, entry, row

**Derived field**:
A Field whose value is computed from other Fields and is never typed by the player.
_Avoid_: Computed stat, calculated field

**Section**:
A top-level block of the Sheet that groups related Rows under one label.
_Avoid_: Group, panel, tab

**Row**:
One line inside a Section: a lone Field, a set of mutually exclusive Fields, a labeled line of Fields in columns, or one Record of a Record list. The smallest unit a player can pin.
_Avoid_: Component, item, widget

**Layout**:
The player's chosen order of Sections and which Rows are pinned. Belongs to the Sheet; the Template supplies the default.
_Avoid_: Arrangement, view

**Pin**:
A player's mark, shown as a star, that lifts a Row to the top of its own Section, or into the Vitals strip when it is one of the first six pinned number Rows. Pinned Rows of a Section appear there in the order they were pinned. A Pin is saved the moment it is toggled, in or out of Arrange mode.
_Avoid_: Favorite, bookmark

**Vitals strip**:
A grid of tiles directly under the Sheet heading showing the first six pinned Rows that are lone number Fields (editable or derived), in pin order. Each tile is the Row itself, edited in place, and a Row shown there is not repeated in its Section; a Section left with no Row is not drawn. Other pinned Rows, and pins past the sixth, stay at the top of their own Section. Not saved: it is derived from the Pins, and it is not drawn while arranging.
_Avoid_: Pinned zone, dashboard, header, summary

**Pinned filter**:
An optional switch in the play view that shows only the pinned Rows, each still in its own Section. Per viewer and per Sheet, remembered in the browser (it stays as the player left it), off until first used and not part of the Layout: it is never saved to the Sheet or counted as unsaved work.
_Avoid_: Pinned tab, pinned zone, favorites view

**Suggested pin**:
A Row a Template recommends as one of a player's first Pins, listed in the Template's `suggested_pins` in suggestion order, at most six. Template content only: the server never pins it for the player, a client offers it (`Pin suggested rows`, on tap only, while nothing is pinned and until it was used once) and the player's Pin is what gets saved.
_Avoid_: Default pin, preset, vital

**Pin mode**:
The lightweight mode, turned on from the Vitals strip, in which every Row shows a star and each toggle is saved at once; nothing is staged and leaving it asks nothing. Never active together with Arrange mode.
_Avoid_: Edit mode, pin editor

**Arrange mode**:
The explicit mode in which a player reorders Sections. Pins can also be toggled outside it (in Pin mode).
_Avoid_: Edit mode, customize mode

**Dice expression**:
A short roll formula made of dice (`NdM`, optionally keeping only the highest or lowest of them), whole numbers and numeric Fields joined by plus or minus, such as `1d20 + stats.dexterity`. On a Section, or on a Row of a table, it can name "this Row" instead of a fixed Field (`1d20 + @`, `1d20 + @.total`), so one Section default serves every Row in it. It attaches to a Row or to a Section (inherited by that Section's Rows). A Template supplies the defaults; a player can replace one, add one where the Template gave none, or turn rolling off for a Row on their own Sheet. The most specific choice wins: the player's for the Row, then the player's "no dice" for the Section, then the Template's for the Row, then the player's expression for the Section, then the Template's for the Section; with none, the Row cannot be rolled. The shorthand `d20` means `1d20`.
_Avoid_: Dice, die, roll formula

**Roll**:
One performance of a Dice expression: each die is thrown, the Fields it names are read at that moment, and the total is shown with each die and number that made it, dropped dice included. Nothing about it is saved: it is not part of the Sheet and there is no Roll history in the first version.
_Avoid_: Throw, dice result

**Roll mode**:
How a d20 Roll is made: normal, advantage (two d20, keep the higher) or disadvantage (two d20, keep the lower). The player picks it at the moment of rolling; it never changes a Field.
_Avoid_: Roll type, keep mode

**Reset action**:
A named action a Template offers that, in one tap, sets several Fields of a Sheet back to values its rules define, such as recovering spent resources after a rest.
_Avoid_: Rest, bulk reset, macro
