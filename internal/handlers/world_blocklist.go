package handlers

// blockedWorldNames is the canonical list of world names that cannot be
// created. Add entries in UPPERCASE — validation normalises before checking.
// Includes reserved names, profanities, and anything else we want to keep off.
var blockedWorldNames = map[string]bool{
	// Reserved / system
	"ADMIN":  true,
	"LOBBY":  true,
	"START":  true,
	"WORLD":  true,
	"SPAWN":  true,
	"DEBUG":  true,
	"TEST_":  true,

	// Profanities (keep this list clean and extend as needed)
	"BITCH":  true,
	"PUSSY":  true,
	"NIGGER": true, // exceeds 8 chars but kept for substring safety if rules change
	"CUNT":   true, // under 5 chars, rejected by length — kept for documentation
	"FUCKS":  true,
	"FUCKED": true,
	"FUCKET": true,
	"ARSES":  true,
	"ARSED":  true,
	"SHITS":  true,
	"SHITE":  true,
	"WANKS":  true,
	"WANKER": true,
	"CUNTS":  true,
	"TWATS":  true,
	"PRICKS": true,
	"SLUTS":  true,
	"WHORES": true,
	"NIGGAS": true,
}
