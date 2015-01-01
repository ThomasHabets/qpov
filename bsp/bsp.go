package bsp

// https://developer.valvesoftware.com/wiki/Source_BSP_File_Format
// http://www.gamers.org/dEngine/quake/spec/quake-spec34/qkspec_4.htm

import (
	"fmt"
	"log"
	"strings"
)

const (
	Version = 29
)

var (
	Verbose = false
)

type dentry struct {
	Offset uint32
	Size   uint32
}

const fileFaceSize = 2 + 2 + 4 + 2 + 2 + 1 + 1 + 2 + 4

const fileTexInfoSize = 3*4 + 4 + 3*4 + 4 + 4 + 4

const fileMiptexSize = 16 + 4 + 4 + 4*4

func (f *RawMipTex) Name() string {
	s := ""
	for _, ch := range f.NameBytes {
		if ch == 0 {
			break
		}
		s = fmt.Sprintf("%s%c", s, ch)
	}
	return s
}

const fileVertexSize = 4 * 3

const fileEdgeSize = 2 + 2

type Vertex struct {
	X, Y, Z float32
}

func (v *Vertex) String() string {
	return fmt.Sprintf("%f,%f,%f", v.X, v.Y, v.Z)
}

type Polygon struct {
	Texture string
	Vertex  []Vertex
}

type BSP struct {
	Raw      *Raw
	StartPos Vertex
}

type Entity struct {
	EntityID int
	Data     map[string]string
	Pos      Vertex
	Angle    Vertex
	Frame    uint8
}

func findStart(es []Entity) Vertex {
	for _, e := range es {
		if e.Data["classname"] == "info_player_start" {
			return parseVertex(e.Data["origin"])
		}
	}
	log.Fatal("can't find start")
	panic("hello")
}

func Load(r myReader) (*BSP, error) {
	raw, err := LoadRaw(r)
	if err != nil {
		return nil, err
	}
	ret := &BSP{
		Raw: raw,
	}
	return ret, nil
}

func (bsp *BSP) Polygons() ([]Polygon, error) {
	polys := []Polygon{}
	return polys, nil
}

func (bsp *BSP) POVTriangleMesh() (string, error) {
	ret := "mesh2 {\n"

	// Add vertices.
	{
		vs := []string{}
		for _, v := range bsp.Raw.Vertex {
			vs = append(vs, fmt.Sprintf("<%s>", v.String()))
		}
		ret += fmt.Sprintf("  vertex_vectors { %d, %s }\n", len(bsp.Raw.Vertex), strings.Join(vs, ","))
	}

	// Add textures.
	ret += "  texture_list { 2, texture{pigment{rgb<1,1,1>}} texture{pigment{rgbf<0,0,1,0.9>}} }\n"

	// Add faces.
	{
		var fstr string
		triCount := 0
		for _, f := range bsp.Raw.Face {
			texture := 0
			texName := bsp.Raw.MipTex[bsp.Raw.TexInfo[f.TexinfoID].TextureID].Name()
			if texName[0] == '*' {
				texture = 1 // water.
			}
			switch texName {
			case "trigger":
				continue
			}

			vs := []uint16{}
			for ledgeNum := f.LEdge; ledgeNum < f.LEdge+uint32(f.LEdgeNum); ledgeNum++ {
				e := bsp.Raw.LEdge[ledgeNum]
				if e == 0 {
					return "", fmt.Errorf("ledge had value 0")
				}
				var vi0, vi1 uint16
				if e < 0 {
					e = -e
					vi1, vi0 = bsp.Raw.Edge[e].From, bsp.Raw.Edge[e].To
				} else {
					vi0, vi1 = bsp.Raw.Edge[e].From, bsp.Raw.Edge[e].To
				}
				_ = vi1
				vs = append(vs, vi0)
			}
			for i := 0; i < len(vs)-2; i++ {
				fstr += fmt.Sprintf("<%d,%d,%d>,%d,\n", vs[0], vs[i+1], vs[i+2], texture)
				triCount++
			}
		}
		ret += fmt.Sprintf("  face_indices { %d, %s }\n", triCount, fstr)
	}

	ret += "  pigment { rgb 1 }\n}\n"
	return ret, nil
}
