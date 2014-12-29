package dem

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
	U_ANGLE2     = 0x0010 // read coord
	U_NOLERP     = 0x0020 // don't interpolate movement
	U_FRAME      = 0x0040 // read one more
	U_SIGNAL     = 0x0080
	U_ANGLE1     = 0x0100 // read coord
	U_ANGLE3     = 0x0200 // read coord
	U_MODEL      = 0x0400 // read byte
	U_COLORMAP   = 0x0800
	U_SKIN       = 0x1000
	U_EFFECTS    = 0x2000
	U_LONGENTITY = 0x4000
)

var (
	Verbose = false
)

type Vertex struct {
	X, Y, Z float32
}

type Demo struct {
	r     io.Reader
	block *bytes.Buffer

	Level     string
	ViewAngle Vertex
	Pos       Vertex
	CameraEnt uint16
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
		r: r,
	}
}

type serverInfo struct {
	// Protocol version of the server. Quake uses the version value 15 and it is not likely, that this will change.
	ServerVersion uint32

	MaxClients uint8 // maximum number of clients in this recording. It is 1 in single player recordings or the number after the -listen command line parameter.

	GameType uint8

	Level string

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

func parseServerInfo(r io.Reader) (serverInfo, error) {
	var si serverInfo
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
			log.Fatalf("Failed to read map name: %v", err)
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
			log.Fatalf("Failed to read map name: %v", err)
		}
		if s == "" {
			break
		}
	}
	return si, nil
}

func readUint8(r io.Reader) (uint8, error) {
	typ := make([]byte, 1, 1)
	if _, err := r.Read(typ); err != nil {
		return 0, err
	}
	return typ[0], nil
}

func readCoord(r io.Reader) (float32, error) {
	t, err := readUint16(r)
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
	t, err := readUint8(r)
	return float32(t) / 256.0 * 360.0, err
}

func readFloat(r io.Reader) (uint32, error) {
	return readUint32(r)
}

func readUint16(r io.Reader) (uint16, error) {
	var ret uint16
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
		//fmt.Printf("Read block of size %d (%x)\n", bh.Blocksize, bh.Blocksize)
		block := make([]byte, bh.Blocksize, bh.Blocksize)
		if _, err := d.r.Read(block); err != nil {
			log.Fatalf("Reading block of size %d: %v", bh.Blocksize, err)
		}
		//log.Printf("Block: %v", block)
		d.block = bytes.NewBuffer(block)
		d.ViewAngle = bh.ViewAngle
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
		log.Printf("Setting %d to %d", i, v)
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
	case 0x11: // set colors
		readUint8(d.block)
		readUint8(d.block)
	case 11:
		si, err := parseServerInfo(d.block)
		if err != nil {
			log.Fatalf("Serverinfo: %v", err)
		}
		d.Level = si.Models[0]

	case 0x1d: // spawnstaticsound
		readCoord(d.block)
		readCoord(d.block)
		readCoord(d.block)
		readUint8(d.block)
		readUint8(d.block)
		readUint8(d.block)
	case 0x19: // This message selects the client state.
		state, _ := readUint8(d.block)
		log.Printf("Set state: %v", state)
	case 0x07: // time
		readFloat(d.block)
	case 0x16: // spawnbaseline
		readUint16(d.block)

		readUint8(d.block)
		readUint8(d.block)
		readUint8(d.block)
		readUint8(d.block)
		readCoord(d.block)
		readAngle(d.block)
		readCoord(d.block)
		readAngle(d.block)
		readCoord(d.block)
		readAngle(d.block)

	case 0x14: // spawnstatic
		readUint8(d.block)
		readUint8(d.block)
		readUint8(d.block)
		readUint8(d.block)
		readCoord(d.block)
		readAngle(d.block)
		readCoord(d.block)
		readAngle(d.block)
		readCoord(d.block)
		readAngle(d.block)
	case 0x20: // CD track
		readUint8(d.block)
		readUint8(d.block)
	case 0x0A: // Camera orientation.
		readAngle(d.block)
		readAngle(d.block)
		readAngle(d.block)
	case 0x05: // Camera pos to this entity.
		d.CameraEnt, _ = readUint16(d.block)
		log.Printf("Camera object changed to %d", d.CameraEnt)
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
	case 0x21: // sell screen
	case 0x08: // Print
		s, _ := readString(d.block)
		log.Printf("Print: %q", s)
	case 0x09: // Stufftext
		s, _ := readString(d.block)
		log.Printf("Stufftext: %q", s)
	case 0x1a: // centerprint
		readString(d.block)
	case 0x1b: // killed monster
	case 0x13: // damage
		readUint8(d.block)
		readUint8(d.block)
		readCoord(d.block)
		readCoord(d.block)
		readCoord(d.block)
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
	default:
		if typ < 0x80 {
			d.block = nil
			return fmt.Errorf("Unknown type %d (0x%x): %v", typ, typ, []byte(tail))
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
		if mask&U_MODEL != 0 {
			readUint8(d.block)
		}
		if mask&U_FRAME != 0 {
			readUint8(d.block)
		}
		if mask&U_COLORMAP != 0 {
			readUint8(d.block)
		}
		if mask&U_SKIN != 0 {
			readUint8(d.block)
		}
		if mask&U_EFFECTS != 0 {
			readUint8(d.block)
		}
		if mask&U_ORIGIN1 != 0 {
			a, _ := readCoord(d.block)
			if ent == d.CameraEnt {
				d.Pos.X = a
			}
		}
		if mask&U_ANGLE1 != 0 {
			a, _ := readAngle(d.block)
			if ent == d.CameraEnt {
				d.ViewAngle.X = a
			}
		}
		if mask&U_ORIGIN2 != 0 {
			a, _ := readCoord(d.block)
			if ent == d.CameraEnt {
				d.Pos.Y = a
			}
		}
		if mask&U_ANGLE2 != 0 {
			a, _ := readAngle(d.block)
			if ent == d.CameraEnt {
				d.ViewAngle.Y = a
			}
		}
		if mask&U_ORIGIN3 != 0 {
			a, _ := readCoord(d.block)
			if ent == d.CameraEnt {
				d.Pos.Z = a
			}
		}
		if mask&U_ANGLE3 != 0 {
			a, _ := readAngle(d.block)
			if ent == d.CameraEnt {
				d.ViewAngle.Z = a
			}
		}
		if Verbose {
			log.Printf("Updating entity %d, remaining: %v", ent, []byte(d.block.String()))
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	return nil
}
