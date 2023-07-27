package dissect

import "github.com/rs/zerolog/log"

func (r *Reader) readLoadout() error {
	available, err := r.Uint32()
	if err != nil {
		return err
	}
	loadoutType, err := r.Bytes(5) // TODO: could be incorrect
	if err != nil {
		return err
	}
	capacity, err := r.Uint32()
	if err != nil {
		return err
	}
	weapon, err := r.Bytes(6) // TODO: could be incorrect
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
		Hex("loadoutType", loadoutType).
		Hex("weapon", weapon).
		Uint32("available", available).
		Uint32("capacity", capacity).
		Msg("loadout")
	return nil
}
