package mdl

// http://tfc.duke.free.fr/coding/mdl-specs-en.html

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"strings"
)

const (
	version = 6
	magic   = 1330660425 // "IDPO"
)

var (
	Verbose = false
)

type Vertex struct {
	X, Y, Z float32
}

type ModelVertex struct {
	Vertex      Vertex
	NormalIndex int
}

func (v *Vertex) String() string {
	return fmt.Sprintf("%g,%g,%g", v.X, v.Y, v.Z)
}
func (v *ModelVertex) String() string {
	return v.Vertex.String()
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
	Vertices []ModelVertex
}

type Model struct {
	Triangles []Triangle
	Frames    []SimpleFrame
}

func (m *Model) POVFrameID(id int) string {
	const useNormals = true

	var ret string
	ret = "mesh2 {\n"

	// Add vertices.
	{
		vs := []string{}
		for _, v := range m.Frames[id].Vertices {
			vs = append(vs, fmt.Sprintf("<%s>", v.String()))
		}
		ret += fmt.Sprintf("  vertex_vectors { %d, %s }\n", len(vs), strings.Join(vs, ","))
	}

	// Add normals.
	if useNormals {
		ns := []string{}
		for _, v := range anorms {
			ns = append(ns, fmt.Sprintf("<%g,%g,%g>", v[0], v[1], v[2]))
		}
		ret += fmt.Sprintf("  normal_vectors { %d, %s }\n", len(ns), strings.Join(ns, ","))
	}

	// Add textures.
	{
		ret += "  texture_list { 1,\n"
		ret += `
    texture {
      pigment { rgb<1,0,0> }
      finish { phong 0.9 phong_size 60 }
    }
`
		ret += "  }\n"
	}

	// Add faces.
	{
		tris := []string{}
		for _, tri := range m.Triangles {
			texture := 0
			tris = append(tris, fmt.Sprintf("<%d,%d,%d>,%d", tri.VertexIndex[0], tri.VertexIndex[1], tri.VertexIndex[2], texture))
		}
		ret += fmt.Sprintf("  face_indices { %d, %s }\n", len(tris), strings.Join(tris, ","))
	}

	// Add normal indices.
	if useNormals {
		vs := []string{}
		for _, v := range m.Frames[id].Vertices {
			vs = append(vs, fmt.Sprintf("%d", v.NormalIndex))
		}
		ret += fmt.Sprintf("  normal_indices { %d, %s }\n", len(vs), strings.Join(vs, ","))
	}

	ret += "rotate rot translate pos}\n"
	return ret
}

type Skin struct {
	Group uint32
	Data  []byte
}

func Load(r myReader) (*Model, error) {
	var h fileHeader
	if Verbose {
		log.Printf("Loading model...")
	}
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if h.Ident != magic {
		return nil, fmt.Errorf("bad magic %08x, want %08x", h.Ident, magic)
	}
	if h.Version != version {
		return nil, fmt.Errorf("bad version %d", h.Version)
	}
	if Verbose {
		log.Printf("Scale: %v", h.Scale)
		log.Printf("Translate: %v", h.Translate)
		log.Printf("Eye: %v", h.EyePosition)
	}

	m := &Model{}

	// Load texture data.
	if Verbose {
		log.Printf("Skins: %v", h.NumSkins)
	}
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
	if Verbose {
		log.Printf("Frames: %v", h.NumFrames)
	}
	for i := uint32(0); i < h.NumFrames; i++ {
		if Verbose {
			log.Printf("  Frame %d", i)
		}
		var typ uint32
		if err := binary.Read(r, binary.LittleEndian, &typ); err != nil {
			return nil, err
		}
		if Verbose {
			log.Printf("    Type %d", typ)
		}
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
			if Verbose {
				log.Printf("    Name: %s", s.Name())
			}
			sf := SimpleFrame{
				Name: s.Name(),
			}
			for n, v := range s.Verts {
				sf.Vertices = append(sf.Vertices, ModelVertex{
					Vertex: Vertex{
						X: (h.Scale.X*float32(v.X) + h.Translate.X),
						Y: (h.Scale.Y*float32(v.Y) + h.Translate.Y),
						Z: (h.Scale.Z*float32(v.Z) + h.Translate.Z),
					},
					NormalIndex: int(v.NormalIndex),
				})
				if Verbose {
					log.Printf("Vert %d: %v -> %v", n, v, sf.Vertices[len(sf.Vertices)-1])
				}
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
