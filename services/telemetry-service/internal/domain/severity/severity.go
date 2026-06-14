package severity

type Level int

const (
	Healthy Level = iota
	Warning
	Critical
)

func (l Level) String() string {
	switch l {
	case Warning:
		return "warning"
	case Critical:
		return "critical"
	default:
		return "healthy"
	}
}

func Parse(s string) Level {
	switch s {
	case "warning":
		return Warning
	case "critical":
		return Critical
	default:
		return Healthy
	}
}
