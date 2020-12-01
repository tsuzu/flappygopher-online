package message

type User struct {
	ID, Name       string
	X16, Y16, VY16 int
	Running        bool
	Score          int
}

const (
	KindUpdate   = "update"
	KindJoin     = "join"
	KindLeave    = "leave"
	KindStanding = "standing"
)

type Result struct {
	Name  string
	Score int
}

type Message struct {
	Kind     string
	User     User
	Standing []Result `json:",omitempty"`
}

func (m *Message) Validate() bool {
	switch m.Kind {
	case KindUpdate:

	case KindLeave:

	case KindStanding:

	default:
		return false
	}

	return true
}
