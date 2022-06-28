package boomer

import "github.com/asaskevich/EventBus"

const (
	EVENT_SPAWN  = "boomer:spawn"
	EVENT_STOP   = "boomer:stop"
	EVENT_QUIT   = "boomer:quit"
	EVENT_REPORT = "boomer:report-to-master"
)

// Events is the global event bus instance.
var Events = EventBus.New()
