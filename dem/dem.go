package dem

// http://www.quakewiki.net/archives/demospecs/dem/dem.html
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
)

const (
	// States
	//	1after model/sound precache, start spawning entities (``prespawn'')
	//	2start initialising light effects
	//	3start 3D rendering

	SU_VIEWHEIGHT  = 0x0001
	SU_IDEALPITCH  = 0x0002
	SU_PUNCH1      = 0x0004
	SU_PUNCH2      = 0x0008
	SU_PUNCH3      = 0x0010
	SU_VELOCITY1   = 0x0020
	SU_VELOCITY2   = 0x0040
	SU_VELOCITY3   = 0x0080
	SU_AIMENT      = 0x0100
	SU_ITEMS       = 0x0200
	SU_ONGROUND    = 0x0400
	SU_INWATER     = 0x0800
	SU_WEAPONFRAME = 0x1000
	SU_ARMOR       = 0x2000
	SU_WEAPON      = 0x4000
	//SU_ = 0x8000

	U_MOREBITS   = 0x0001
	U_ORIGIN1    = 0x0002
	U_ORIGIN2    = 0x0004
	U_ORIGIN3    = 0x0008
	U_ANGLE2     = 0x0010 // read angle
	U_NOLERP     = 0x0020 // don't interpolate movement
	U_FRAME      = 0x0040 // read one more
	U_SIGNAL     = 0x0080
	U_ANGLE1     = 0x0100 // read angle
	U_ANGLE3     = 0x0200 // read angle
	U_MODEL      = 0x0400 // read byte
	U_COLORMAP   = 0x0800
	U_SKIN       = 0x1000
	U_EFFECTS    = 0x2000
	U_LONGENTITY = 0x4000

	TE_SPIKE        = 0
	TE_SUPERSPIKE   = 1
	TE_GUNSHOT      = 2
	TE_EXPLOSION    = 3
	TE_TAREXPLOSION = 4
	TE_LIGHTNING1   = 5
	TE_LIGHTNING2   = 6
	TE_WIZSPIKE     = 7
	TE_KNIGHTSPIKE  = 8
	TE_LIGHTNING3   = 9
	TE_LAVASPLASH   = 10
	TE_TELEPORT     = 11
	TE_EXPLOSION2   = 12

	maxEntities = 1000
)

var (
	Verbose = false
)

type Vertex struct {
	X, Y, Z float32
}

func (v *Vertex) String() string {
	return fmt.Sprintf("%f,%f,%f", v.X, v.Y, v.Z)
}

type Entity struct {
	Pos   Vertex
	Angle Vertex
	Model uint8
	Frame uint8
	// TODO: skin.
}

type Demo struct {
	r     io.Reader
	block *bytes.Buffer

	Level     string
	ViewAngle Vertex
	Pos       Vertex
	CameraEnt uint16

	ServerInfo ServerInfo
	Entities   []Entity
}

type blockHeader struct {
	Blocksize uint32
	ViewAngle Vertex
}

type Message struct {
}

func Open(r io.Reader) *Demo {
	for {
		ch, err := readUint8(r)
		if err != nil {
			log.Fatal(err)
		}
		if Verbose {
			log.Printf("Read first line char: %02x", ch)
		}
		if ch == '\n' {
			break
		}
	}
	return &Demo{
		r:        r,
		Entities: make([]Entity, maxEntities, maxEntities),
	}
}

type ServerInfo struct {
	// Protocol version of the server. Quake uses the version value 15 and it is not likely, that this will change.
	ServerVersion uint32

	MaxClients uint8 // maximum number of clients in this recording. It is 1 in single player recordings or the number after the -listen command line parameter.

	GameType uint8

	Level  string
	Models []string
	Sounds []string
}

func readString(r io.Reader) (string, error) {
	b := make([]byte, 1, 1)
	ret := ""
	for {
		if _, err := r.Read(b); err != nil {
			return "", err
		}
		if b[0] == 0 {
			return ret, nil
		}
		ret = fmt.Sprintf("%s%c", ret, b[0])
	}
}

func parseServerInfo(r io.Reader) (ServerInfo, error) {
	var si ServerInfo
	var err error
	if err := binary.Read(r, binary.LittleEndian, &si.ServerVersion); err != nil {
		log.Fatalf("Reading server version: %v", err)
	}
	if si.ServerVersion != 15 {
		return si, fmt.Errorf("ServerVersion != 15: %v", si.ServerVersion)
	}
	if err := binary.Read(r, binary.LittleEndian, &si.MaxClients); err != nil {
		log.Fatalf("Reading max clients: %v", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &si.GameType); err != nil {
		log.Fatalf("Reading gametype: %v", err)
	}
	si.Level, err = readString(r)
	if err != nil {
		log.Fatalf("Failed to read map name: %v", err)
	}
	log.Printf("Level %q", si.Level)

	// Read model list.
	for {
		s, err := readString(r)
		if err != nil {
			log.Fatalf("Failed to read model name: %v", err)
		}
		if s == "" {
			break
		}
		si.Models = append(si.Models, s)
	}

	// Read sound list.
	for {
		s, err := readString(r)
		if err != nil {
			log.Fatalf("Failed to read sound name: %v", err)
		}
		if s == "" {
			break
		}
		si.Sounds = append(si.Sounds, s)
	}
	return si, nil
}

func readInt8(r io.Reader) (int8, error) {
	var ret int8
	if err := binary.Read(r, binary.LittleEndian, &ret); err != nil {
		return 0, err
	}
	return ret, nil
}
func readUint8(r io.Reader) (uint8, error) {
	typ := make([]byte, 1, 1)
	if _, err := r.Read(typ); err != nil {
		return 0, err
	}
	return typ[0], nil
}

func readCoord(r io.Reader) (float32, error) {
	t, err := readInt16(r)
	return float32(t) * 0.125, err
}

func readUint32(r io.Reader) (uint32, error) {
	var ret uint32
	if err := binary.Read(r, binary.LittleEndian, &ret); err != nil {
		return 0, err
	}
	return ret, nil
}

func readAngle(r io.Reader) (float32, error) {
	t, err := readInt8(r)
	return float32(t) / 256.0 * 360.0, err
}

func readFloat(r io.Reader) (float32, error) {
	var ret float32
	if err := binary.Read(r, binary.LittleEndian, &ret); err != nil {
		return 0, err
	}
	return ret, nil
}

func readUint16(r io.Reader) (uint16, error) {
	var ret uint16
	if err := binary.Read(r, binary.LittleEndian, &ret); err != nil {
		return 0, err
	}
	return ret, nil
}

func readInt16(r io.Reader) (int16, error) {
	var ret int16
	if err := binary.Read(r, binary.LittleEndian, &ret); err != nil {
		return 0, err
	}
	return ret, nil
}

func (d *Demo) Read() error {
	if d.block == nil || len(d.block.String()) == 0 {
		var bh blockHeader
		if err := binary.Read(d.r, binary.LittleEndian, &bh); err != nil {
			return err
		}
		//log.Printf("Read block of size %d (%x)\n", bh.Blocksize, bh.Blocksize)
		block := make([]byte, bh.Blocksize, bh.Blocksize)
		if _, err := d.r.Read(block); err != nil {
			log.Fatalf("Reading block of size %d: %v", bh.Blocksize, err)
		}
		//log.Printf("Block: %v", block)
		d.block = bytes.NewBuffer(block)
		//d.ViewAngle = bh.ViewAngle
	}
	tail := d.block.String()
	typ, err := readUint8(d.block)
	if err != nil {
		log.Fatalf("reading type: %v", err)
	}
	if Verbose {
		log.Printf("message type %d (0x%02x)", typ, typ)
	}
	switch typ {
	case 0x01: // NOP
	case 0x02: // disconnect
	case 0x03: // player state
		i, _ := readUint8(d.block)
		v, _ := readUint32(d.block)
		if Verbose {
			log.Printf("Setting %d to %d", i, v)
		}
	case 0x05: // Camera pos to this entity.
		d.CameraEnt, _ = readUint16(d.block)
		if Verbose {
			log.Printf("Camera object changed to %d", d.CameraEnt)
		}
	case 0x06: // Play sound.
		mask, _ := readUint8(d.block)
		if mask&0x1 != 0 {
			readUint8(d.block)
		}
		if mask&0x2 != 0 {
			readUint8(d.block)
		}
		readUint16(d.block)
		readUint8(d.block)
		readCoord(d.block)
		readCoord(d.block)
		readCoord(d.block)
	case 0x07: // time
		t, _ := readFloat(d.block)
		if Verbose {
			log.Printf("Time: %f", t)
		}
	case 0x08: // Print
		s, _ := readString(d.block)
		log.Printf("Print: %q", s)
	case 0x09: // Stufftext
		s, _ := readString(d.block)
		log.Printf("Stufftext: %q", s)
	case 0x0A: // Camera orientation.
		x, _ := readAngle(d.block)
		y, _ := readAngle(d.block)
		z, _ := readAngle(d.block)
		log.Printf("Camera orientation changed to %f %f %f", x, y, z)
	case 0x0b:
		log.Printf("SERVERINFO")
		var err error
		d.ServerInfo, err = parseServerInfo(d.block)
		if err != nil {
			log.Fatalf("Serverinfo: %v", err)
		}
		d.Level = d.ServerInfo.Models[0]
	case 0x0c: // light style
		readUint8(d.block)
		readString(d.block)
	case 0x0d: // set player name
		i, _ := readUint8(d.block)
		name, _ := readString(d.block)
		log.Printf("Setting player %d name to %q", i, name)
	case 0x0e: // set frags
		readUint8(d.block)
		readUint16(d.block)
	case 0x0F: // client data
		mask, _ := readUint16(d.block)
		if Verbose {
			log.Printf("Mask: %04x", mask)
		}
		if mask&SU_VIEWHEIGHT != 0 {
			readUint8(d.block)
		}
		if mask&SU_IDEALPITCH != 0 {
			readUint8(d.block)
		}
		if mask&SU_PUNCH1 != 0 {
			readUint8(d.block)
		}
		if mask&SU_VELOCITY1 != 0 {
			readUint8(d.block)
		}
		if mask&SU_PUNCH2 != 0 {
			readUint8(d.block)
		}
		if mask&SU_VELOCITY2 != 0 {
			readUint8(d.block)
		}
		if mask&SU_PUNCH3 != 0 {
			readUint8(d.block)
		}
		if mask&SU_VELOCITY3 != 0 {
			readUint8(d.block)
		}
		if mask&SU_AIMENT != 0 {
		}
		if mask&SU_ONGROUND != 0 {
		}
		if mask&SU_INWATER != 0 {
		}
		if mask&SU_ITEMS != 0 {
			readUint32(d.block)
		}
		if mask&SU_WEAPONFRAME != 0 {
			readUint8(d.block)
		}
		if mask&SU_ARMOR != 0 {
			readUint8(d.block)
		}
		if mask&SU_WEAPON != 0 {
			readUint8(d.block)
		}
		health, _ := readUint16(d.block)
		if Verbose {
			log.Printf("Health: %v", health)
		}

		ammo, _ := readUint8(d.block)
		if Verbose {
			log.Printf("Ammo: %v", ammo)
		}

		shells, _ := readUint8(d.block)
		if Verbose {
			log.Printf("Shells: %v", shells)
		}

		ammo_nails, _ := readUint8(d.block)
		if Verbose {
			log.Printf("Nails: %v", ammo_nails)
		}

		ammo_rockets, _ := readUint8(d.block)
		if Verbose {
			log.Printf("Rockets: %v", ammo_rockets)
		}

		ammo_cells, _ := readUint8(d.block)
		if Verbose {
			log.Printf("Cells: %v", ammo_cells)
		}

		weapon, _ := readUint8(d.block)
		if Verbose {
			log.Printf("Weapon: %v", weapon)
			log.Printf("After clientdata: %v", []byte(d.block.String()))
		}

	case 0x11: // set colors
		readUint8(d.block)
		readUint8(d.block)
	case 0x12: // particle
		readCoord(d.block)
		readCoord(d.block)
		readCoord(d.block)
		readInt8(d.block)
		readInt8(d.block)
		readInt8(d.block)
		readUint8(d.block)
		readUint8(d.block)
	case 0x13: // damage
		readUint8(d.block)
		readUint8(d.block)
		readCoord(d.block)
		readCoord(d.block)
		readCoord(d.block)
	case 0x14: // spawnstatic
		model, _ := readUint8(d.block)
		frame, _ := readUint8(d.block)
		color, _ := readUint8(d.block)
		skin, _ := readUint8(d.block)
		x, _ := readCoord(d.block)
		a, _ := readAngle(d.block)
		y, _ := readCoord(d.block)
		b, _ := readAngle(d.block)
		z, _ := readCoord(d.block)
		c, _ := readAngle(d.block)
		if Verbose {
			log.Printf("Spawning static %f,%f,%f: %d %d %d %d %f %f %f", x, y, z, model, frame, color, skin, a, b, c)
		}
	case 0x16: // spawnbaseline
		ent, _ := readUint16(d.block)

		model, _ := readUint8(d.block)
		frame, _ := readUint8(d.block)
		color, _ := readUint8(d.block)
		skin, _ := readUint8(d.block)

		x, _ := readCoord(d.block)
		a, _ := readAngle(d.block)
		y, _ := readCoord(d.block)
		b, _ := readAngle(d.block)
		z, _ := readCoord(d.block)
		c, _ := readAngle(d.block)

		if Verbose {
			log.Printf("Spawning baseline %d at <%f,%f,%f>: %d (%s) %d %d %d %f %f %f", ent, x, y, z, model, d.ServerInfo.Models[model], frame, color, skin, a, b, c)
		}
		if Verbose && ent == 40 {
			log.Printf("  Model: %v", d.Entities[model])
		}
		d.Entities[ent].Pos.X, d.Entities[ent].Pos.Y, d.Entities[ent].Pos.Z = x, y, z
		//TODO: why not? d.Entities[ent].Angle.X, d.Entities[ent].Angle.Y, d.Entities[ent].Angle.Z = a, b, c
		d.Entities[ent].Model = model
		d.Entities[ent].Frame = frame
	case 0x17: // temp entity
		entityType, _ := readUint8(d.block)
		if Verbose {
			log.Printf("Temp entity type %d", entityType)
		}
		switch entityType {
		// TE_KNIGHT_SPIKE
		case TE_SPIKE, TE_SUPERSPIKE, TE_GUNSHOT, TE_EXPLOSION, TE_TAREXPLOSION, TE_WIZSPIKE, TE_LAVASPLASH, TE_TELEPORT:
			readCoord(d.block)
			readCoord(d.block)
			readCoord(d.block)

		case TE_LIGHTNING1, TE_LIGHTNING2, TE_LIGHTNING3: // TE_BEAM
			readUint16(d.block)
			readCoord(d.block)
			readCoord(d.block)
			readCoord(d.block)
			readCoord(d.block)
			readCoord(d.block)
			readCoord(d.block)
		case TE_EXPLOSION2:
			readCoord(d.block)
			readCoord(d.block)
			readCoord(d.block)
			readUint8(d.block)
			readUint8(d.block)
		default:
			return fmt.Errorf("bad temp ent type")
		}
	case 0x19: // This message selects the client state.
		state, _ := readUint8(d.block)
		if Verbose {
			log.Printf("Set state: %v", state)
		}
	case 0x1a: // centerprint
		readString(d.block)
	case 0x1b: // killed monster
	case 0x1d: // spawnstaticsound
		readCoord(d.block)
		readCoord(d.block)
		readCoord(d.block)
		readUint8(d.block)
		readUint8(d.block)
		readUint8(d.block)
	case 0x1e: // intermission
		readString(d.block)
	case 0x20: // CD track
		readUint8(d.block)
		readUint8(d.block)
	case 0x21: // sell screen
	default:
		if typ < 0x80 {
			d.block = nil
			return fmt.Errorf("unknown type %d (0x%x), tail: %v", typ, typ, []byte(tail))
		}
		mask := uint16(typ & 0x7F)
		if mask&U_MOREBITS != 0 {
			t, _ := readUint8(d.block)
			mask |= uint16(t) << 8
		}
		if Verbose {
			log.Printf("Update packet mask %04x: %v", mask, []byte(d.block.String()))
		}
		var ent uint16
		if mask&U_LONGENTITY != 0 {
			ent, _ = readUint16(d.block)
		} else {
			e, _ := readUint8(d.block)
			ent = uint16(e)
		}
		if Verbose {
			log.Printf("Update to entity %d", ent)
		}
		debugEnt := false
		if Verbose && ent == 40 {
			debugEnt = true
		}
		if mask&U_MODEL != 0 {
			a, err := readUint8(d.block)
			if err != nil {
				log.Fatal(err)
			}
			if Verbose && d.Entities[ent].Model != a {
				log.Printf("  Update %d: Model %d (%q -> %q)", ent, a, d.ServerInfo.Models[d.Entities[ent].Model], d.ServerInfo.Models[a])
			}
			d.Entities[ent].Frame = 0
			d.Entities[ent].Model = a
		}
		if mask&U_FRAME != 0 {
			a, err := readUint8(d.block)
			if err != nil {
				log.Fatal(err)
			}
			d.Entities[ent].Frame = a
			if debugEnt {
				log.Printf("  Set frame of %d to %d", ent, a)
			}
		}
		if mask&U_COLORMAP != 0 {
			a, _ := readUint8(d.block)
			if debugEnt {
				log.Printf("  Update %d: Colormap %d", ent, a)
			}
		}
		if mask&U_SKIN != 0 {
			a, _ := readUint8(d.block)
			if debugEnt {
				log.Printf("  Update %d: Skin %d", ent, a)
			}
		}
		if mask&U_EFFECTS != 0 {
			a, _ := readUint8(d.block)
			if debugEnt {
				log.Printf("  Update %d: Effects %d", ent, a)
			}
		}
		if mask&U_ORIGIN1 != 0 {
			a, err := readCoord(d.block)
			if err != nil {
				log.Fatal(err)
			}
			d.Entities[ent].Pos.X = a
			if debugEnt {
				log.Printf("  Update %d: Pos X %f", ent, a)
			}
		}
		if mask&U_ANGLE1 != 0 {
			a, _ := readAngle(d.block)
			d.Entities[ent].Angle.X = a
			if debugEnt {
				log.Printf("  Update %d: Angle 1 %f", ent, a)
			}
		}
		if mask&U_ORIGIN2 != 0 {
			a, _ := readCoord(d.block)
			d.Entities[ent].Pos.Y = a
			if debugEnt {
				log.Printf("  Update %d: Pos Y %f", ent, a)
			}
		}
		if mask&U_ANGLE2 != 0 {
			a, _ := readAngle(d.block)
			d.Entities[ent].Angle.Z = a
			if debugEnt {
				log.Printf("  Update %d: Angle 2 %f", ent, a)
			}
		}
		if mask&U_ORIGIN3 != 0 {
			a, _ := readCoord(d.block)
			d.Entities[ent].Pos.Z = a
			if debugEnt {
				log.Printf("  Update %d: Pos Z %f", ent, a)
			}
		}
		if mask&U_ANGLE3 != 0 {
			a, _ := readAngle(d.block)
			d.Entities[ent].Angle.Y = a
			if debugEnt {
				log.Printf("  Update %d: Angle 3 %f", ent, a)
			}
		}
		if debugEnt {
			log.Printf("  Data for %d: %v", ent, d.Entities[ent])
		}
	}
	if d.Entities != nil {
		d.Pos.X = d.Entities[d.CameraEnt].Pos.X
		d.Pos.Y = d.Entities[d.CameraEnt].Pos.Y
		d.Pos.Z = d.Entities[d.CameraEnt].Pos.Z
		d.ViewAngle.X = d.Entities[d.CameraEnt].Angle.X
		d.ViewAngle.Y = d.Entities[d.CameraEnt].Angle.Y
		d.ViewAngle.Z = d.Entities[d.CameraEnt].Angle.Z
	}
	if err != nil {
		log.Fatal(err)
	}
	return nil
}
