package dissect

type UtilityEvent struct {
	Operator      string  `json:"operator"`
	Username      string  `json:"username"`
	UtilityType   string  `json:"utilityType"`
	Action        string  `json:"action"`
	TimeInSeconds float64 `json:"time"`
}

type CameraDestruction struct {
	DestroyedBy   string  `json:"destroyedBy"`
	TimeInSeconds float64 `json:"time"`
}

type DroneDestruction struct {
	DestroyedBy   string  `json:"destroyedBy"`
	TimeInSeconds float64 `json:"time"`
}

type Reinforcement struct {
	Username      string  `json:"username"`
	TimeInSeconds float64 `json:"time"`
}

type BarricadePlace struct {
	Username      string  `json:"username"`
	TimeInSeconds float64 `json:"time"`
}

type GadgetDeployment struct {
	Username      string  `json:"username"`
	Operator      string  `json:"operator"`
	GadgetType    string  `json:"gadgetType"`
	TimeInSeconds float64 `json:"time"`
}

type PlayerPosition struct {
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	Z     float32 `json:"z"`
	Yaw   float32 `json:"yaw,omitempty"`
	Pitch float32 `json:"pitch,omitempty"`
}

type EntityPositions struct {
	EntityID  byte             `json:"entityId"`
	Username  string           `json:"username,omitempty"`
	Team      string           `json:"team"`
	Positions []PlayerPosition `json:"positions"`
}

type LocationEvent struct {
	Username      string  `json:"username,omitempty"`
	Room          string  `json:"room"`
	TimeInSeconds float64 `json:"time"`
}
