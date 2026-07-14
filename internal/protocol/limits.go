package protocol

const (
	MaxWebSocketMessageBytes   = 256 * 1024
	MaxUINodeDepth             = 12
	MaxUIChildren              = 64
	MaxUIItems                 = 64
	MaxUIGradientStops         = 8
	MaxGridRows                = 20
	MaxGridCols                = 12
	MaxSingleLineTextRunes     = 128
	MaxMultilineTextRunes      = 2048
	MaxChromeTextRunes         = 128
	MaxRenderMessagesPerSecond = 20
	MaxNotificationsPerMinute  = 60
	MaxStateOpsPerBatch        = 64
	MaxStateKeyBytes           = 128
	MaxStateValueJSONBytes     = 64 * 1024
	MaxStateBatchJSONBytes     = 256 * 1024
	MaxScopedStateJSONBytes    = 512 * 1024
	MaxTransitionsPerResponse  = 4
	MaxActionsPerResponse      = 1
	MaxActionChain             = 4
	MaxActionInputJSONBytes    = 64 * 1024
)
