package action

type Type string

const (
	ImagingActionType  Type = "Imaging"
	CleaningActionType Type = "Cleaning"
)

type Status struct {
	Type  Type   `json:"type"`
	Done  bool   `json:"done"`
	Error string `json:"error,omitempty"`
}

type Action interface {
	Do()
	Status() (*Status, error)
}
