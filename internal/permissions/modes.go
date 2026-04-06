package permissions

// Mode represents the permission mode
type Mode int

const (
	ModeDefault Mode = iota
	ModeAcceptEdits
	ModeDontAsk
	ModeBypassPermissions
	ModePlan
)

// String returns the mode name
func (m Mode) String() string {
	switch m {
	case ModeDefault:
		return "default"
	case ModeAcceptEdits:
		return "acceptEdits"
	case ModeDontAsk:
		return "dontAsk"
	case ModeBypassPermissions:
		return "bypassPermissions"
	case ModePlan:
		return "plan"
	default:
		return "unknown"
	}
}

// ParseMode parses a mode string
func ParseMode(s string) Mode {
	switch s {
	case "default":
		return ModeDefault
	case "acceptEdits":
		return ModeAcceptEdits
	case "dontAsk":
		return ModeDontAsk
	case "bypassPermissions", "bypass":
		return ModeBypassPermissions
	case "plan":
		return ModePlan
	default:
		return ModeDefault
	}
}

// ShouldAsk returns whether this mode requires user confirmation
func (m Mode) ShouldAsk() bool {
	switch m {
	case ModeDefault, ModePlan:
		return true
	case ModeAcceptEdits, ModeDontAsk, ModeBypassPermissions:
		return false
	default:
		return true
	}
}

// AllowEdit returns whether edits are allowed without confirmation
func (m Mode) AllowEdit() bool {
	return m == ModeAcceptEdits || m == ModeBypassPermissions
}

// AllowBypass returns whether permission checks can be skipped
func (m Mode) AllowBypass() bool {
	return m == ModeBypassPermissions
}

// IsPlanMode returns whether this is a planning-only mode
func (m Mode) IsPlanMode() bool {
	return m == ModePlan
}
