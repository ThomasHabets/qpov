// Package dem processes Quake DEM files.
package dem

// QPov
//
// Copyright (C) Thomas Habets <thomas@habets.se> 2015
// https://github.com/ThomasHabets/qpov
//
//   This program is free software; you can redistribute it and/or modify
//   it under the terms of the GNU General Public License as published by
//   the Free Software Foundation; either version 2 of the License, or
//   (at your option) any later version.
//
//   This program is distributed in the hope that it will be useful,
//   but WITHOUT ANY WARRANTY; without even the implied warranty of
//   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//   GNU General Public License for more details.
//
//   You should have received a copy of the GNU General Public License along
//   with this program; if not, write to the Free Software Foundation, Inc.,
//   51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
//
// http://demospecs.half-empty.de/lmpc-alpha/
// http://www.quakewiki.net/archives/demospecs/dem/dem.html
//
// TODO
// * Remove all log.Fatals
// * Check errors from all read*() calls.
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"

	"github.com/ThomasHabets/qpov/pkg/bsp"
)

const (
	version15         = 15
	versionQuakeSpasm = 666

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

	// Effects
	EF_MUZZLEFLASH = 0x2

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
	TE_BEAM         = 13
	TE_IMPLOSION    = 14
	TE_RAILTRAIL    = 15

	maxEntities = 1000
)

var (
	Verbose  = false
	debugEnt = uint16(65000)
)

type Vertex struct {
	X, Y, Z float32
}

func (v *Vertex) String() string {
	return fmt.Sprintf("%f,%f,%f", v.X, v.Y, v.Z)
}

type Entity struct {
	Pos     Vertex
	Angle   Vertex
	Model   uint8
	Frame   uint8
	Skin    uint8
	Color   int
	Visible bool
}

type Demo struct {
	r     io.Reader
	block *bytes.Buffer

	Level      string
	CameraEnt  uint16
	viewAngle  Vertex
	BlockCount int

	ServerInfo ServerInfo
	Entities   []Entity
	Time       float32
}

type BlockHeader struct {
	Blocksize uint32
	ViewAngle Vertex
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
	if (si.ServerVersion != version15) && (si.ServerVersion != versionQuakeSpasm) {
		return si, fmt.Errorf("ServerVersion %v not supported", si.ServerVersion)
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

	// Read model list.
	for {
		s, err := readString(r)
		if err != nil {
			log.Fatalf("Failed to read model name: %v", err)
		}
		if s == "" {
			break
		}
		if Verbose {
			log.Printf("  ----> Loading model %d: %q", len(si.Models), s)
		}
		si.Models = append(si.Models, s)
		if len(si.Models) == 1 {
			si.Models = append(si.Models, s)
		}
	}

	// Read sound list.
	si.Sounds = []string{""} // First sound is empty.
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

type SoundEvent struct {
	Time  float64
	Sound MsgPlaySound
}
type State struct {
	Time       float64
	Entities   []Entity
	SeenEntity map[uint16]bool

	// 2 Means render 3D.
	ClientState int

	CameraEnt          int
	CameraViewAngle    Vertex
	CameraSetViewAngle bool // If CameraOrientation has been set, ignore header.
	ViewAngle          Vertex
	ServerInfo         ServerInfo
	Level              *bsp.BSP

	Sounds []SoundEvent
}

func NewState() *State {
	return &State{
		Entities:   make([]Entity, 1000, 1000),
		SeenEntity: make(map[uint16]bool),
	}
}

func (s *State) Copy() *State {
	n := NewState()
	n.Time = s.Time
	for i := range n.Entities {
		n.Entities[i] = s.Entities[i]
	}
	n.CameraViewAngle = s.CameraViewAngle
	n.CameraSetViewAngle = s.CameraSetViewAngle
	n.CameraEnt = s.CameraEnt
	n.ClientState = s.ClientState
	n.ViewAngle = s.ViewAngle
	n.SeenEntity = s.SeenEntity
	n.ServerInfo = s.ServerInfo
	n.Level = s.Level
	return n
}

type Message interface {
	Apply(*State)
}

type MsgIntermission struct {
	Text string
}

func (m *MsgIntermission) Apply(s *State) {
	s.CameraSetViewAngle = true
	s.ViewAngle = s.CameraViewAngle
}

type MsgFinale struct {
	Text string
}

func (m *MsgFinale) Apply(s *State) {
	s.CameraSetViewAngle = true
	s.ViewAngle = s.CameraViewAngle
}

type MsgNop struct{}

func (m MsgNop) Apply(s *State) {}

type MsgLightStyle struct {
	Index uint8
	Style string
}

func (m MsgLightStyle) Apply(s *State) {}

type MsgPlayerName struct {
	Index uint8
	Name  string
}

func (m MsgPlayerName) Apply(s *State) {}

type MsgFrags struct {
	Player uint8
	Frags  uint16
}

func (m MsgFrags) Apply(s *State) {}

type MsgClientState struct {
	State uint8
}

func (m MsgClientState) Apply(s *State) {
	s.ClientState = int(m.State)
}

type MsgUpdate struct {
	Entity                             uint16
	X, Y, Z                            *float32
	A, B, C                            *float32
	Model, Skin, Color, Effects, Frame *uint8
}

func (m MsgUpdate) Apply(s *State) {
	s.SeenEntity[m.Entity] = true
	if m.Entity == debugEnt {
		log.Printf("Debugent MsgUpdate: %+v", m)
	}
	if m.X != nil {
		s.Entities[m.Entity].Pos.X = *m.X
	}
	if m.Y != nil {
		s.Entities[m.Entity].Pos.Y = *m.Y
	}
	if m.Z != nil {
		s.Entities[m.Entity].Pos.Z = *m.Z
	}
	if m.A != nil {
		s.Entities[m.Entity].Angle.X = *m.A
	}
	if m.B != nil {
		s.Entities[m.Entity].Angle.Y = *m.B
	}
	if m.C != nil {
		s.Entities[m.Entity].Angle.Z = *m.C
	}

	if m.Model != nil {
		if m.Entity == debugEnt {
			log.Printf("  Model; %d", *m.Model)
		}
		s.Entities[m.Entity].Model = *m.Model
		s.Entities[m.Entity].Skin = 0
		s.Entities[m.Entity].Color = 0
		s.Entities[m.Entity].Frame = 0
	}
	if m.Skin != nil {
		s.Entities[m.Entity].Skin = *m.Skin
	}
	if m.Color != nil {
		s.Entities[m.Entity].Color = int(*m.Color)
	}
	if m.Effects != nil {
		// TODO s.Entities[m.Entity].Effects = int(*m.Effects)
	}
	if m.Frame != nil {
		s.Entities[m.Entity].Frame = *m.Frame
	}

	if false {
		if int(m.Entity) == s.CameraEnt {
			s.ViewAngle = s.Entities[m.Entity].Angle
		}
	}
}

type MsgSpawnBaseline struct {
	Entity                    uint16
	X, Y, Z                   float32
	A, B, C                   float32
	Model, Frame, Color, Skin uint8
}

func (m MsgSpawnBaseline) Apply(s *State) {
	s.Entities[m.Entity].Pos.X = m.X
	s.Entities[m.Entity].Pos.Y = m.Y
	s.Entities[m.Entity].Pos.Z = m.Z
	s.Entities[m.Entity].Angle.X = m.A
	s.Entities[m.Entity].Angle.Y = m.B
	s.Entities[m.Entity].Angle.Z = m.C
	s.Entities[m.Entity].Model = m.Model
	s.Entities[m.Entity].Frame = m.Frame
	s.Entities[m.Entity].Color = int(m.Color)
	s.Entities[m.Entity].Skin = m.Skin
}

type MsgDisconnect struct{}

func (m MsgDisconnect) Apply(s *State) {}

type MsgPlayerState struct {
	Key   uint8
	Value uint32
}

func (m MsgPlayerState) Apply(s *State) {}

type MsgPlaySound struct {
	Sound         int
	X, Y, Z       float32
	Channel       int // Auto, weapon, voice, item, body
	Entity        uint16
	EntityChannel int
	Volume        int
	Attenuation   int
}

func (m MsgPlaySound) Apply(s *State) {
	s.Sounds = append(s.Sounds, SoundEvent{
		Time:  s.Time,
		Sound: m,
	})
}

type MsgCameraPos struct {
	Entity uint16
}

func (m MsgCameraPos) Apply(s *State) {
	s.CameraEnt = int(m.Entity)
}

type MsgCameraOrientation struct {
	X, Y, Z float32
}

func (m MsgCameraOrientation) Apply(s *State) {
	s.CameraViewAngle.X = m.X
	s.CameraViewAngle.Y = m.Y
	s.CameraViewAngle.Z = m.Z
	s.ViewAngle = s.CameraViewAngle
}

func (si *ServerInfo) Apply(s *State) {
	s.ServerInfo = ServerInfo(*si)
}

type MsgTime float32

func (m *MsgTime) Apply(s *State) { s.Time = float64(*m) }

type Block struct {
	Header BlockHeader
	buf    *bytes.Buffer
}

func (block *Block) Messages() ([]Message, error) {
	messages := []Message{}
	for {
		if len(block.buf.String()) == 0 {
			return messages, nil
		}
		m, err := block.DecodeMessage()
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
}

func (block *Block) DecodeMessage() (Message, error) {
	typ, err := readUint8(block.buf)
	if err != nil {
		log.Fatalf("reading type: %v", err)
	}
	if Verbose {
		log.Printf("message type %d (0x%02x)", typ, typ)
	}
	switch typ {
	case 0x01: // NOP
		return &MsgNop{}, nil
	case 0x02: // disconnect
		return &MsgDisconnect{}, nil
	case 0x03: // player state
		r := &MsgPlayerState{}
		r.Key, _ = readUint8(block.buf)
		r.Value, _ = readUint32(block.buf)
		return r, nil
	case 0x05: // Camera pos to this entity.
		r := &MsgCameraPos{}
		r.Entity, _ = readUint16(block.buf)
		return r, nil
	case 0x06: // Play sound.
		snd := MsgPlaySound{}
		mask, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		if mask&0x1 != 0 {
			t, err := readUint8(block.buf) // vol
			if err != nil {
				return nil, err
			}
			snd.Volume = int(t)
		}
		if mask&0x2 != 0 {
			t, err := readUint8(block.buf) // attenuation
			if err != nil {
				return nil, err
			}
			snd.Attenuation = int(t)
		}
		{
			entityChannel, err := readUint16(block.buf)
			if err != nil {
				return nil, err
			}
			snd.Channel = int(entityChannel) & 0x07
			snd.Entity = (uint16(entityChannel) >> 3) & 0x1FFF
			if debugEnt == snd.Entity {
				log.Printf("Entity %d made a sound", snd.Entity)
			}
		}
		{
			t, err := readUint8(block.buf)
			if err != nil {
				return nil, err
			}
			snd.Sound = int(t)
		}
		snd.X, err = readCoord(block.buf)
		if err != nil {
			return nil, err
		}
		snd.Y, err = readCoord(block.buf)
		if err != nil {
			return nil, err
		}
		snd.Z, err = readCoord(block.buf)
		if err != nil {
			return nil, err
		}
		return &snd, nil

	case 0x07: // time
		t, err := readFloat(block.buf)
		t2 := MsgTime(t)
		return &t2, err

	case 0x08: // Print
		s, err := readString(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Print: %q", s)
		}
	case 0x09: // Stufftext
		s, err := readString(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Stufftext: %q", s)
		}
	case 0x0A: // Camera orientation.
		x, err := readAngle(block.buf)
		if err != nil {
			return nil, err
		}
		y, err := readAngle(block.buf)
		if err != nil {
			return nil, err
		}
		z, err := readAngle(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Camera orientation changed to %f %f %f", x, y, z)
		}
		return &MsgCameraOrientation{
			X: x,
			Y: y,
			Z: z,
		}, nil
	case 0x0b: // serverinfo
		if Verbose {
			log.Printf("SERVERINFO")
		}
		si, err := parseServerInfo(block.buf)
		if err != nil {
			log.Fatalf("Serverinfo: %v", err)
		}
		return &si, nil
	case 0x0c: // light style
		styleIndex, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		style, err := readString(block.buf)
		if err != nil {
			return nil, err
		}
		return &MsgLightStyle{
			Index: styleIndex,
			Style: style,
		}, nil
	case 0x0d: // set player name
		i, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		name, err := readString(block.buf)
		if err != nil {
			return nil, err
		}
		if false {
			log.Printf("Setting player %d name to %q", i, name)
		}
		return &MsgPlayerName{
			Index: i,
			Name:  name,
		}, nil
	case 0x0e: // set frags
		player, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		frags, err := readUint16(block.buf)
		if err != nil {
			return nil, err
		}
		return &MsgFrags{
			Player: player,
			Frags:  frags,
		}, nil
	case 0x0F: // client data
		mask, err := readUint16(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Mask: %04x", mask)
		}
		if mask&SU_VIEWHEIGHT != 0 {
			viewOffsetZ, _ := readUint8(block.buf)
			// TODO: Use this to offset camera in Z axis.
			_ = viewOffsetZ
		}
		if mask&SU_IDEALPITCH != 0 {
			readUint8(block.buf)
		}
		if mask&SU_PUNCH1 != 0 {
			readUint8(block.buf)
		}
		if mask&SU_VELOCITY1 != 0 {
			readUint8(block.buf)
		}
		if mask&SU_PUNCH2 != 0 {
			readUint8(block.buf)
		}
		if mask&SU_VELOCITY2 != 0 {
			readUint8(block.buf)
		}
		if mask&SU_PUNCH3 != 0 {
			readUint8(block.buf)
		}
		if mask&SU_VELOCITY3 != 0 {
			readUint8(block.buf)
		}
		if mask&SU_AIMENT != 0 {
		}
		if mask&SU_ONGROUND != 0 {
		}
		if mask&SU_INWATER != 0 {
			// TODO: blend some blue.
		}
		if mask&SU_ITEMS != 0 {
			readUint32(block.buf)
		}
		if mask&SU_WEAPONFRAME != 0 {
			readUint8(block.buf)
		}
		if mask&SU_ARMOR != 0 {
			readUint8(block.buf)
		}
		if mask&SU_WEAPON != 0 {
			readUint8(block.buf)
		}
		health, err := readUint16(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Health: %v", health)
		}

		ammo, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Ammo: %v", ammo)
		}

		shells, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Shells: %v", shells)
		}

		ammo_nails, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Nails: %v", ammo_nails)
		}

		ammo_rockets, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Rockets: %v", ammo_rockets)
		}

		ammo_cells, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Cells: %v", ammo_cells)
		}

		weapon, err := readUint8(block.buf)
		if err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("Weapon: %v", weapon)
		}

	case 0x10: // stopsound
		readUint16(block.buf)

	case 0x11: // set colors
		readUint8(block.buf) // player
		readUint8(block.buf) // color
	case 0x12: // particle
		readCoord(block.buf) // origin...
		readCoord(block.buf)
		readCoord(block.buf)
		readInt8(block.buf) // velocity...
		readInt8(block.buf)
		readInt8(block.buf)
		readUint8(block.buf) // count
		readUint8(block.buf) // color (chunk 0, blood 73, barrel 75 and thunderbolt 225)
	case 0x13: // damage
		readUint8(block.buf) // armor
		readUint8(block.buf) // health
		readCoord(block.buf) // origin of hit...
		readCoord(block.buf)
		readCoord(block.buf)
	case 0x14: // spawnstatic
		model, _ := readUint8(block.buf)
		frame, _ := readUint8(block.buf)
		color, _ := readUint8(block.buf)
		skin, _ := readUint8(block.buf)
		x, _ := readCoord(block.buf)
		a, _ := readAngle(block.buf)
		y, _ := readCoord(block.buf)
		b, _ := readAngle(block.buf)
		z, _ := readCoord(block.buf)
		c, _ := readAngle(block.buf)
		if Verbose {
			log.Printf("Spawning static %f,%f,%f: %d %d %d %d %f %f %f", x, y, z, model, frame, color, skin, a, b, c)
		}
		// TODO: Spawn something static.
	case 0x16: // spawnbaseline
		r := &MsgSpawnBaseline{}
		r.Entity, _ = readUint16(block.buf)

		r.Model, _ = readUint8(block.buf)
		r.Frame, _ = readUint8(block.buf)
		r.Color, _ = readUint8(block.buf)
		r.Skin, _ = readUint8(block.buf)

		r.X, _ = readCoord(block.buf)
		r.A, _ = readAngle(block.buf)
		r.Y, _ = readCoord(block.buf)
		r.B, _ = readAngle(block.buf)
		r.Z, _ = readCoord(block.buf)
		r.C, _ = readAngle(block.buf)
		return r, nil

	case 0x17: // temp entity
		entityType, _ := readUint8(block.buf)
		if Verbose {
			log.Printf("Temp entity type %d", entityType)
		}
		switch entityType {
		case TE_SPIKE, TE_SUPERSPIKE, TE_GUNSHOT, TE_EXPLOSION, TE_TAREXPLOSION, TE_WIZSPIKE, TE_LAVASPLASH, TE_TELEPORT, TE_KNIGHTSPIKE, TE_IMPLOSION:
			readCoord(block.buf) // origin...
			readCoord(block.buf)
			readCoord(block.buf)

		case TE_LIGHTNING1, TE_LIGHTNING2, TE_LIGHTNING3, TE_BEAM, TE_RAILTRAIL:
			ent, _ := readUint16(block.buf)
			if debugEnt == ent {
				log.Printf("Lightning from ent %d", ent)
			}
			readCoord(block.buf) // from...
			readCoord(block.buf)
			readCoord(block.buf)
			readCoord(block.buf) // to...
			readCoord(block.buf)
			readCoord(block.buf)
		case TE_EXPLOSION2:
			readCoord(block.buf) // origin...
			readCoord(block.buf)
			readCoord(block.buf)
			readUint8(block.buf) // color
			readUint8(block.buf) // range
		default:
			return nil, fmt.Errorf("bad temp ent type %d", entityType)
		}
		// TODO: spawn temp entity.
	case 0x18: // setpause
		readUint8(block.buf)
	case 0x19: // signonnum
		state, _ := readUint8(block.buf)
		if Verbose {
			log.Printf("Set state: %v", state)
		}
		return &MsgClientState{State: state}, nil
	case 0x1a: // centerprint
		readString(block.buf)
	case 0x1b: // killed monster
	case 0x1c: // found secret
	case 0x1d: // spawnstaticsound
		readCoord(block.buf) // origin
		readCoord(block.buf)
		readCoord(block.buf)
		readUint8(block.buf) // num
		readUint8(block.buf) // vol
		readUint8(block.buf) // attenuation
	case 0x1e: // intermission
		t, _ := readString(block.buf)
		return &MsgIntermission{Text: t}, nil
	case 0x1f: // finale - end screen
		t, _ := readString(block.buf)
		return &MsgFinale{Text: t}, nil
	case 0x20: // CD track
		readUint8(block.buf) // from track
		readUint8(block.buf) // to track
	case 0x21: // sell screen
	default:
		m := &MsgUpdate{}
		if typ < 0x80 {
			block.buf = nil
			return nil, fmt.Errorf("unknown type %d (0x%x), tail: %v", typ, typ, []byte(block.buf.String()))
		}
		mask := uint16(typ & 0x7F)
		if mask&U_MOREBITS != 0 {
			t, _ := readUint8(block.buf)
			mask |= uint16(t) << 8
		}
		if Verbose {
			log.Printf("Update packet mask %04x: %v", mask, []byte(block.buf.String()))
		}
		if mask&U_LONGENTITY != 0 {
			m.Entity, _ = readUint16(block.buf)
		} else {
			e, _ := readUint8(block.buf)
			m.Entity = uint16(e)
		}
		if m.Entity == debugEnt {
			log.Printf("DebugEnt mask: %04x", mask)
		}
		if mask&U_MODEL != 0 {
			a, err := readUint8(block.buf)
			if err != nil {
				log.Fatal(err)
			}
			m.Model = &a
		}
		if mask&U_FRAME != 0 {
			a, err := readUint8(block.buf)
			if err != nil {
				log.Fatal(err)
			}
			m.Frame = &a
		}
		if mask&U_COLORMAP != 0 {
			a, _ := readUint8(block.buf)
			m.Color = &a
		}
		if mask&U_SKIN != 0 {
			a, _ := readUint8(block.buf)
			m.Skin = &a
		}
		if mask&U_EFFECTS != 0 {
			a, _ := readUint8(block.buf)
			m.Effects = &a
			if *m.Effects&0xfd != 0 {
				log.Printf("Entity %v effect %v", m.Entity, a)
			}
		}
		if mask&U_ORIGIN1 != 0 {
			a, err := readCoord(block.buf)
			if err != nil {
				log.Fatal(err)
			}
			m.X = &a
		}
		if mask&U_ANGLE1 != 0 {
			a, _ := readAngle(block.buf)
			m.A = &a
		}
		if mask&U_ORIGIN2 != 0 {
			a, _ := readCoord(block.buf)
			m.Y = &a
		}
		if mask&U_ANGLE2 != 0 {
			a, _ := readAngle(block.buf)
			m.B = &a
		}
		if mask&U_ORIGIN3 != 0 {
			a, _ := readCoord(block.buf)
			m.Z = &a
		}
		if mask&U_ANGLE3 != 0 {
			a, _ := readAngle(block.buf)
			m.C = &a
		}
		return m, nil
	}
	return &MsgNop{}, nil
}

func (d *Demo) ReadBlock() (*Block, error) {
	block := &Block{}
	if err := binary.Read(d.r, binary.LittleEndian, &block.Header); err != nil {
		return nil, err
	}
	data := make([]byte, block.Header.Blocksize, block.Header.Blocksize)
	if _, err := d.r.Read(data); err != nil {
		return nil, fmt.Errorf("reading block of size %d: %v", block.Header.Blocksize, err)
	}
	block.buf = bytes.NewBuffer(data)
	return block, nil
}
