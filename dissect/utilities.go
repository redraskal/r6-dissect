package dissect

import (
	"github.com/rs/zerolog/log"
)

func (r *Reader) recordScoreBasedDestruction(username string, destructionType string, timeInSeconds float64) {
	operator := r.getOperatorByUsername(username)

	if destructionType == "camera" {
		r.CameraDestructions = append(r.CameraDestructions, CameraDestruction{
			DestroyedBy:   username,
			TimeInSeconds: timeInSeconds,
		})
		log.Debug().
			Str("username", username).
			Float64("time", timeInSeconds).
			Msg("camera_destruction_detected")
	} else if destructionType == "drone" {
		r.DroneDestructions = append(r.DroneDestructions, DroneDestruction{
			DestroyedBy:   username,
			TimeInSeconds: timeInSeconds,
		})
		log.Debug().
			Str("username", username).
			Float64("time", timeInSeconds).
			Msg("drone_destruction_detected")
	} else if destructionType == "reinforcement" {
		r.Reinforcements = append(r.Reinforcements, Reinforcement{
			Username:      username,
			TimeInSeconds: timeInSeconds,
		})
		log.Debug().
			Str("username", username).
			Float64("time", timeInSeconds).
			Msg("reinforcement_detected")
	} else if destructionType == "barricade" {
		r.Barricades = append(r.Barricades, BarricadePlace{
			Username:      username,
			TimeInSeconds: timeInSeconds,
		})
		log.Debug().
			Str("username", username).
			Float64("time", timeInSeconds).
			Msg("barricade_detected")
	} else if destructionType == "gadget_deploy" {
		r.GadgetDeployments = append(r.GadgetDeployments, GadgetDeployment{
			Username:      username,
			Operator:      operator,
			GadgetType:    ClassifyGadgetByOperator(operator),
			TimeInSeconds: timeInSeconds,
		})
		log.Debug().
			Str("username", username).
			Str("operator", operator).
			Float64("time", timeInSeconds).
			Msg("gadget_deployment_detected")
	} else {
		r.UtilityEvents = append(r.UtilityEvents, UtilityEvent{
			Operator:      operator,
			Username:      username,
			UtilityType:   ClassifyGadgetByOperator(operator),
			Action:        "destroyed",
			TimeInSeconds: timeInSeconds,
		})
		log.Debug().
			Str("username", username).
			Str("operator", operator).
			Float64("time", timeInSeconds).
			Msg("utility_destruction_detected")
	}
}

func (r *Reader) getOperatorByUsername(username string) string {
	for _, p := range r.Header.Players {
		if p.Username == username {
			return p.Operator.String()
		}
	}
	return "Unknown"
}

func (r *Reader) getPlayerTeamRole(username string) TeamRole {
	for _, p := range r.Header.Players {
		if p.Username == username {
			return r.Header.Teams[p.TeamIndex].Role
		}
	}
	return ""
}

func ClassifyGadgetByOperator(operator string) string {
	gadgetMap := map[string]string{
		// Pathfinders - SAS
		"Sledge":   "Tactical Breaching Hammer",
		"Thatcher": "EMP Grenade",
		"Smoke":    "Remote Gas Grenade",
		"Mute":     "Signal Disruptor",
		// Pathfinders - FBI SWAT
		"Ash":      "Breaching Round",
		"Thermite": "Exothermic Charge",
		"Castle":   "Armor Panel",
		"Pulse":    "Heartbeat Sensor",
		// Pathfinders - GIGN
		"Twitch":   "Shock Drone",
		"Montagne": "Extendable Shield",
		"Doc":      "Stim Pistol",
		"Rook":     "Armor Pack",
		// Pathfinders - Spetsnaz
		"Glaz":     "Flip Sight",
		"Fuze":     "Cluster Charge",
		"Kapkan":   "Entry Denial Device",
		"Tachanka": "Shumikha Launcher",
		// Pathfinders - GSG 9
		"Blitz":  "Flash Shield",
		"IQ":     "Electronics Detector",
		"Jager":  "Active Defense System",
		"Bandit": "Shock Wire",
		// Pathfinders - Recruit
		"Recruit": "Recruit",
		// Year 1
		"Buck":       "Skeleton Key",
		"Frost":      "Welcome Mat",
		"Blackbeard": "Rifle Shield",
		"Valkyrie":   "Black Eye",
		"Capitao":    "Tactical Crossbow",
		"Caveira":    "Silent Step",
		"Hibana":     "X-KAIROS",
		"Echo":       "Yokai",
		// Year 2
		"Jackal":   "Eyenox Model III",
		"Mira":     "Black Mirror",
		"Ying":     "Candela",
		"Lesion":   "Gu Mine",
		"Zofia":    "KS79 LIFELINE",
		"Ela":      "Grzmot Mine",
		"Dokkaebi": "Logic Bomb",
		"Vigil":    "ERC-7",
		// Year 3
		"Lion":     "EE-ONE-D",
		"Finka":    "Adrenal Surge",
		"Maestro":  "Evil Eye",
		"Alibi":    "Prisma",
		"Maverick": "Breaching Torch",
		"Clash":    "CCE Shield",
		"Nomad":    "Airjab Launcher",
		"Kaid":     "Rtila Electroclaw",
		// Year 4
		"Gridlock": "Trax Stingers",
		"Mozzie":   "Pest Launcher",
		"Nokk":     "HEL Presence Reduction",
		"Warden":   "Glance Smart Glasses",
		"Amaru":    "Garra Hook",
		"Goyo":     "Volcán Canister",
		"Kali":     "LV Explosive Lance",
		"Wamai":    "Mag-NET",
		// Year 5
		"Iana":   "Gemini Replicator",
		"Oryx":   "Remah Dash",
		"Ace":    "S.E.L.M.A. Aqua Breacher",
		"Melusi": "Banshee Sonic Defense",
		"Zero":   "ARGUS Camera",
		"Aruni":  "Surya Laser Gate",
		// Year 6
		"Flores":      "RCE-RATERO Drone",
		"Thunderbird": "Kóna Station",
		"Osa":         "Talon-8 Clear Shield",
		"Thorn":       "Razorbloom Shell",
		// Year 7
		"Azami": "Kiba Barrier",
		"Sens":  "R.O.U. Projector System",
		"Grim":  "Kawan Hive Launcher",
		"Solis": "SPEC-IO Electro-Sensor",
		// Year 8
		"Brava":   "Kludge Drone",
		"Fenrir":  "Dread Mine",
		"Ram":     "BU-GI Auto Breacher",
		"Tubarao": "Zoto Canister",
		// Year 9
		"Deimos":  "DeathMark",
		"Striker": "Field Presence System",
		"Sentry":  "CENTINEL System",
		"Skopos":  "PANOPTES",
		// Year 10
		"Rauora": "Tūpara Quake Charge",
		"Denari": "T.R.I.P. Connector",
		// Year 11
		"SolidSnake": "Soliton Radar Mk. III",
	}
	if g, ok := gadgetMap[operator]; ok {
		return g
	}
	return "Gadget"
}
