package mdl

// http://tfc.duke.free.fr/coding/mdl-specs-en.html

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
)

const (
	version = 6
	magic   = 1330660425 // "IDPO"
)

type Vertex struct {
	X, Y, Z float32
}

func (v *Vertex) String() string {
	return fmt.Sprintf("%f, %f, %f", v.X, v.Y, v.Z)
}

type myReader interface {
	io.Reader
	io.Seeker
}

type fileHeader struct {
	Ident uint32 // magic number: "IDPO"

	Version uint32 /* version: 6 */

	Scale          Vertex /* scale factor */
	Translate      Vertex /* translation vector */
	BoundinGradius float32
	EyePosition    Vertex /* eyes' position */

	NumSkins   uint32 /* number of textures */
	SkinWidth  uint32 /* texture width */
	SkinHeight uint32 /* texture height */

	NumVertices  uint32 /* number of vertices */
	NumTriangles uint32 /* number of triangles */
	NumFrames    uint32 /* number of frames */

	Synctype uint32 /* 0 = synchron, 1 = random */
	Flags    uint32 /* state flag */
	Size     float32
}

type TexCoords struct {
	A, B, C uint32
}

type Triangle struct {
	FacesFront  uint32
	VertexIndex [3]uint32
}

type SimpleFrame struct {
	Name     string
	Vertices []Vertex
}

type Model struct {
	Triangles []Triangle
	Frames    []SimpleFrame
}

func (m *Model) POVFrameID(id int) string {
	var ret string
	frame := m.Frames[id]
	for _, tri := range m.Triangles {
		ret += fmt.Sprintf(`polygon {
  3,
  <%v>,
  <%v>,
  <%v>
  finish {
    ambient 0.1
    diffuse 0.6
  }
  pigment {
    Red
  }
}
`,
			frame.Vertices[tri.VertexIndex[0]].String(),
			frame.Vertices[tri.VertexIndex[1]].String(),
			frame.Vertices[tri.VertexIndex[2]].String(),
		)
	}
	return ret
}

type Skin struct {
	Group uint32
	Data  []byte
}

func Load(r myReader) (*Model, error) {
	var h fileHeader
	log.Printf("Loading model...")
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if h.Ident != magic {
		return nil, fmt.Errorf("bad magic")
	}
	if h.Version != version {
		return nil, fmt.Errorf("bad version %d", h.Version)
	}
	log.Printf("Scale: %v", h.Scale)
	log.Printf("Translate: %v", h.Translate)
	log.Printf("Eye: %v", h.EyePosition)

	m := &Model{}

	// Load texture data.
	log.Printf("Skins: %v", h.NumSkins)
	for i := uint32(0); i < h.NumSkins; i++ {
		skin := Skin{
			Data: make([]byte, h.SkinWidth*h.SkinHeight),
		}
		if err := binary.Read(r, binary.LittleEndian, &skin.Group); err != nil {
			return nil, err
		}
		if _, err := r.Read(skin.Data); err != nil {
			return nil, err
		}
	}

	// Load texcoords.
	tcoords := make([]TexCoords, h.NumVertices)
	if err := binary.Read(r, binary.LittleEndian, &tcoords); err != nil {
		return nil, err
	}

	// Load triangles.
	m.Triangles = make([]Triangle, h.NumTriangles)
	if err := binary.Read(r, binary.LittleEndian, &m.Triangles); err != nil {
		return nil, err
	}

	// Load frames.
	log.Printf("Frames: %v", h.NumFrames)
	for i := uint32(0); i < h.NumFrames; i++ {
		log.Printf("  Frame %d", i)
		var typ uint32
		if err := binary.Read(r, binary.LittleEndian, &typ); err != nil {
			return nil, err
		}
		log.Printf("    Type %d", typ)
		if typ == 0 {
			s := simpleFrame{
				Verts: make([]modelVertex, h.NumVertices, h.NumVertices),
			}
			if err := binary.Read(r, binary.LittleEndian, &s.Bboxmin); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &s.Bboxmax); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &s.NameBytes); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &s.Verts); err != nil {
				return nil, err
			}
			log.Printf("    Name: %s", s.Name())
			sf := SimpleFrame{
				Name: s.Name(),
			}
			for n, v := range s.Verts {
				sf.Vertices = append(sf.Vertices, Vertex{
					X: (h.Scale.X*float32(v.X) + h.Translate.X),
					Y: (h.Scale.Y*float32(v.Y) + h.Translate.Y),
					Z: (h.Scale.Z*float32(v.Z) + h.Translate.Z),
				})
				log.Printf("Vert %d: %v -> %v", n, v, sf.Vertices[len(sf.Vertices)-1])
			}
			m.Frames = append(m.Frames, sf)
		} else {
			return nil, fmt.Errorf("non-simple frames not implemented")
		}
	}
	return m, nil
}

type simpleFrame struct {
	Bboxmin   modelVertex /* bouding box min */
	Bboxmax   modelVertex /* bouding box max */
	NameBytes [16]byte
	Verts     []modelVertex /* vertex list of the frame */
}

func (f *simpleFrame) Name() string {
	s := ""
	for _, ch := range f.NameBytes {
		if ch == 0 {
			break
		}
		s = fmt.Sprintf("%s%c", s, ch)
	}
	return s
}

type modelVertex struct {
	X, Y, Z     uint8
	NormalIndex uint8
}
