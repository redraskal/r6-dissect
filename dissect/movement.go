package dissect

import "bytes"

type MovementUpdates struct {
	Username string           `json:"username"`
	Data     []MovementUpdate `json:"data"`
}

type MovementUpdate struct {
	X float32
	Y float32
	Z float32
}

// TODO: there appears to be multiple types of movement updates (movement only, rotation only (?), movement + rotation (?))
// that can be identified by 2 bytes
// gotta implement the other types of movement updates
var movementType = []byte{0xC0, 0x3F}

func readMovement(r *Reader) error {
	t, err := r.Bytes(2)
	if err != nil {
		return err
	}
	if !bytes.Equal(t, movementType) {
		return nil
	}
	x, err := r.Float32()
	if err != nil {
		return err
	}
	y, err := r.Float32()
	if err != nil {
		return err
	}
	z, err := r.Float32()
	if err != nil {
		return err
	}
	if len(r.Movement) == 0 {
		// TODO: how to ID the player and the timestamp??
		// maybe there is a consistent update rate?
		r.Movement = append(r.Movement, MovementUpdates{
			Username: "N/A",
			Data:     []MovementUpdate{},
		})
	}
	r.Movement[0].Data = append(r.Movement[0].Data, MovementUpdate{
		X: x,
		Y: y,
		Z: z,
	})
	return nil
}
