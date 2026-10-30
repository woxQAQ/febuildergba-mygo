package text

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
)

var ErrText = fault.Code{
	ID:   "game.invalid_text",
	Kind: fault.InvalidData,
}

var ErrNameEncoding = fault.Code{
	ID:   "game.invalid_name_encoding",
	Kind: fault.InvalidInput,
}

var ErrTextEncoding = fault.Code{
	ID:   "game.invalid_text_encoding",
	Kind: fault.InvalidInput,
}

var ErrTextControl = fault.Code{
	ID:   "game.unsupported_text_control",
	Kind: fault.Unsupported,
}

var ErrTextArgument = fault.Code{
	ID:   "game.invalid_text_argument",
	Kind: fault.InvalidInput,
}
