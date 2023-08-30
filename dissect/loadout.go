package dissect

import "github.com/rs/zerolog/log"

func (r *Reader) readAmmo() error {
	available, err := r.Uint32()
	if err != nil {
		return err
	}
	if err = r.Skip(5); err != nil {
		return err
	}
	capacity, err := r.Uint32()
	if err != nil {
		return err
	}
	if err := r.Seek([]byte{0xD9, 0x9D}); err != nil {
		return err
	}
	id, err := r.Bytes(4)
	if err != nil {
		return err
	}
	i := r.PlayerIndexByID(id)
	log.Debug().
		Str("username", r.Header.Players[i].Username).
		Uint32("available", available).
		Uint32("capacity", capacity).
		Msg("ammo")
	return nil
}
