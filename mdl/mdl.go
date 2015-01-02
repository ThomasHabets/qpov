package mdl

// http://tfc.duke.free.fr/coding/mdl-specs-en.html

import (
	"encoding/binary"
	"fmt"
	"image"
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

type RawHeader struct {
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
	Onseam uint32
	S, T   uint32
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
	Header        RawHeader
	Skins         []image.Image
	Triangles     []Triangle
	TextureCoords []TexCoords
	Frames        []SimpleFrame
}

func (m *Model) POVFrameID(id int, skin string) string {
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

	// Add texture coordinates.
	if skin != "" {
		vs := []string{}
		for _, v := range m.TextureCoords {
			// Add every texture coordinate twice. Once for normal vertices,
			// and once for backfacing onseam.
			s := (float64(v.S)) / float64(m.Header.SkinWidth)
			t := (float64(v.T)) / float64(m.Header.SkinHeight)
			vs = append(vs, fmt.Sprintf("<%v,%v>", s, t),
				fmt.Sprintf("<%v,%v>", s+0.5, t))
		}
		ret += fmt.Sprintf("  uv_vectors { %d, %s }\n", len(vs), strings.Join(vs, ","))
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
	if skin != "" {
		ret += "  texture_list { 1,\n"
		ret += fmt.Sprintf(`
    texture {
      uv_mapping
      pigment {
        image_map {
          png %s
          // TODO: interpolate 2
        }
        rotate <180,0,0>
      }
      //finish { specular 0.1 phong_size 60 }
    }
`, skin)
		ret += "  }\n"
	} else {
		ret += "  texture_list { 1, texture { pigment { rgb<1,0,0> } } }\n"
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

	// Add texture indeces indices.
	if skin != "" {
		vs := []string{}
		for _, tri := range m.Triangles {
			ind := []int{0, 0, 0}
			for i := 0; i < 3; i++ {
				ind[i] = int(tri.VertexIndex[i] * 2)
				if tri.FacesFront == 0 {
					if m.TextureCoords[tri.VertexIndex[i]].Onseam > 0 {
						ind[i]++
					}
				}
			}
			vs = append(vs, fmt.Sprintf("<%v,%v,%v>", ind[0], ind[1], ind[2]))
			//_ = tri
			//vs = append(vs, "<0,0,0>")
		}
		ret += fmt.Sprintf("  uv_indices { %d, %s }\n", len(vs), strings.Join(vs, ","))
	}

	ret += "rotate rot translate pos}\n"
	return ret
}

type Skin struct {
	Group uint32
	Data  []uint8
}

func Load(r myReader) (*Model, error) {
	m := &Model{}
	if Verbose {
		log.Printf("Loading model...")
	}
	if err := binary.Read(r, binary.LittleEndian, &m.Header); err != nil {
		return nil, err
	}
	if m.Header.Ident != magic {
		return nil, fmt.Errorf("bad magic %08x, want %08x", m.Header.Ident, magic)
	}
	if m.Header.Version != version {
		return nil, fmt.Errorf("bad version %d", m.Header.Version)
	}
	if Verbose {
		log.Printf("Scale: %v", m.Header.Scale)
		log.Printf("Translate: %v", m.Header.Translate)
		log.Printf("Eye: %v", m.Header.EyePosition)
	}

	// Load texture data.
	if Verbose {
		log.Printf("Skins: %v", m.Header.NumSkins)
	}
	for i := uint32(0); i < m.Header.NumSkins; i++ {
		skin := Skin{
			Data: make([]uint8, m.Header.SkinWidth*m.Header.SkinHeight),
		}
		if err := binary.Read(r, binary.LittleEndian, &skin.Group); err != nil {
			return nil, err
		}
		if _, err := r.Read(skin.Data); err != nil {
			return nil, err
		}
		img := image.NewPaletted(image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: int(m.Header.SkinWidth), Y: int(m.Header.SkinHeight)}}, quakePalette)
		for n, b := range skin.Data {
			img.SetColorIndex(n%int(m.Header.SkinWidth), n/int(m.Header.SkinWidth), b)
		}
		m.Skins = append(m.Skins, img)
	}

	// Load texcoords.
	m.TextureCoords = make([]TexCoords, m.Header.NumVertices)
	if err := binary.Read(r, binary.LittleEndian, &m.TextureCoords); err != nil {
		return nil, err
	}

	// Load triangles.
	m.Triangles = make([]Triangle, m.Header.NumTriangles)
	if err := binary.Read(r, binary.LittleEndian, &m.Triangles); err != nil {
		return nil, err
	}

	// Load frames.
	if Verbose {
		log.Printf("Frames: %v", m.Header.NumFrames)
	}
	for i := uint32(0); i < m.Header.NumFrames; i++ {
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
				Verts: make([]modelVertex, m.Header.NumVertices, m.Header.NumVertices),
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
						X: (m.Header.Scale.X*float32(v.X) + m.Header.Translate.X),
						Y: (m.Header.Scale.Y*float32(v.Y) + m.Header.Translate.Y),
						Z: (m.Header.Scale.Z*float32(v.Z) + m.Header.Translate.Z),
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
