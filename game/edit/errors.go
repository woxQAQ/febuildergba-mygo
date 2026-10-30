package edit

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
)

var ErrVersion = fault.Code{
	ID:   "game.unsupported_version",
	Kind: fault.Unsupported,
}

var ErrLayout = fault.Code{
	ID:   "game.invalid_layout",
	Kind: fault.InvalidData,
}

var ErrField = fault.Code{
	ID:   "game.invalid_field",
	Kind: fault.InvalidInput,
}

var ErrValue = fault.Code{
	ID:   "game.invalid_value",
	Kind: fault.InvalidInput,
}
