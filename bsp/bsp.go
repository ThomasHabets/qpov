package bsp

// https://developer.valvesoftware.com/wiki/Source_BSP_File_Format
// http://www.gamers.org/dEngine/quake/spec/quake-spec34/qkspec_4.htm

import (
	"fmt"
	"regexp"
	"strconv"
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

const fileModelSize = 2*3*4 + 3*4 + 4*4 + 3*4

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
	Raw *Raw
}

type Entity struct {
	EntityID int
	Data     map[string]string
	Pos      Vertex
	Angle    Vertex
	Frame    uint8
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

func ModelPrefix(s string) string {
	re := regexp.MustCompile(`[/.*-]`)
	return "modelprefix_" + re.ReplaceAllString(s, "_")
}

func (bsp *BSP) Polygons() ([]Polygon, error) {
	polys := []Polygon{}
	return polys, nil
}

func (bsp *BSP) POVLights() string {
	ret := []string{}
	for _, ent := range bsp.Raw.Entities {
		if ent.Data["classname"] == "light" {
			brightness, err := strconv.ParseFloat(ent.Data["light"], 64)
			if err != nil {
				brightness = 200.0
			}
			brightness /= 200.0 // 200.0 is Quake baseline.
			ret = append(ret, fmt.Sprintf("light_source {<%v> Gray50*%g}", ent.Pos.String(), brightness))
		}
	}
	return strings.Join(ret, "\n")
}

func (bsp *BSP) POVTriangleMesh(prefix string, withTextures bool) (string, error) {
	var ret string
	for modelNumber := range bsp.Raw.Models {
		//  TODO: Prune vertexes and everything else not needed for this model.
		ret += fmt.Sprintf("#macro %s_%d(pos,rot)\n", prefix, modelNumber)

		triangles, err := bsp.makeTriangles(modelNumber)
		if err != nil {
			return "", nil
		}
		if len(triangles) == 0 {
			ret += "#end\n"
			continue
		}
		ret += "object { mesh2 {\n"
		// Add vertices.
		{
			vs := []string{}
			for _, v := range bsp.Raw.Vertex {
				vs = append(vs, fmt.Sprintf("<%s>", v.String()))
			}
			ret += fmt.Sprintf("  vertex_vectors { %d, %s }\n", len(bsp.Raw.Vertex), strings.Join(vs, ","))
		}

		// Add texture coordinates.
		if withTextures {
			vs := []string{}
			vs = append(vs, "<0,0>", "<0,1>", "<1,0>")
			for _, tri := range triangles {
				//ti := bsp.Raw.TexInfo[tri.Face.TexinfoID]
				vs = append(vs, fmt.Sprintf("<%v,%v>", 0, 0))
				vs = append(vs, fmt.Sprintf("<%v,%v>",
					0.1*(bsp.Raw.Vertex[tri.B].X-bsp.Raw.Vertex[tri.A].X),
					0.1*(bsp.Raw.Vertex[tri.B].Y-bsp.Raw.Vertex[tri.A].Y),
				))
				vs = append(vs, fmt.Sprintf("<%v,%v>",
					0.1*(bsp.Raw.Vertex[tri.C].X-bsp.Raw.Vertex[tri.A].X),
					0.1*(bsp.Raw.Vertex[tri.C].Y-bsp.Raw.Vertex[tri.A].Y),
				))
			}
			ret += fmt.Sprintf("  uv_vectors { %d, %s }\n", len(vs), strings.Join(vs, ","))
		}

		// TODO: add normals.

		// Add textures.
		if withTextures {
			//var textures []string
			ret += `
  texture_list { 2,
    texture {
      uv_mapping
      pigment {
        image_map {
          png "/home/habets/go/progs/player.mdl/skin_0.png"
          interpolate 2
        }
        rotate <180,0,0>
      }
    }
    texture {
      normal { bumps 0.08 scale <1,0.25,0.35>*1 turbulence 0.6 }
      pigment { rgbf<0,0,1,0.2> }
      finish {
        reflection 0.3
        diffuse 0.55
      }
    }
  }
`
		} else {
			ret += "  texture_list { 2,\n"
			ret += "    texture{pigment{Gray25}}"

			ret += `
    texture {
      normal { bumps 0.08 scale <1,0.25,0.35>*1 turbulence 0.6 }
      pigment { rgbf<0,0,1,0.2> }
      finish {
        reflection 0.3
        diffuse 0.55
      }
    }`
			ret += "  }\n"
		}

		// Add faces.
		{
			var tris []string
			for _, tri := range triangles {
				texture := 0
				texName := bsp.Raw.MipTex[bsp.Raw.TexInfo[tri.Face.TexinfoID].TextureID].Name()
				if texName[0] == '*' {
					texture = 1 // water.
				}
				tris = append(tris, fmt.Sprintf("<%d,%d,%d>,%d", tri.A, tri.B, tri.C, texture))
			}
			ret += fmt.Sprintf("  face_indices { %d, %s }\n", len(tris), strings.Join(tris, ","))
		}

		// TODO: Add normal indices.

		// Add texture coord indices.
		if withTextures {
			var tris []string
			for n, tri := range triangles {
				if false {
					tris = append(tris, fmt.Sprintf("<%v,%v,%v>", tri.A, tri.B, tri.C))
				} else {
					tris = append(tris, fmt.Sprintf("<%d,%d,%d>", n, n+1, n+2))
				}
			}
			ret += fmt.Sprintf("  uv_indices { %d, %s }\n", len(tris), strings.Join(tris, ","))
		}

		ret += "  pigment { rgb 1 }\n} rotate rot translate pos}\n#end\n"
	}
	return ret, nil
}

type Triangle struct {
	Face    RawFace
	A, B, C int
}

func (bsp *BSP) makeTriangles(modelNumber int) ([]Triangle, error) {
	skipFace := make(map[int]bool)
	for nm, m := range bsp.Raw.Models {
		for n := int(m.FaceID); n < int(m.FaceID+m.FaceNum); n++ {
			if nm != modelNumber {
				skipFace[n] = true
			}
		}
	}

	tris := []Triangle{}
	for fn, f := range bsp.Raw.Face {
		if skipFace[fn] {
			continue
		}
		texName := bsp.Raw.MipTex[bsp.Raw.TexInfo[f.TexinfoID].TextureID].Name()
		switch texName {
		case "trigger":
			continue
		}
		vs := []uint16{}
		for ledgeNum := f.LEdge; ledgeNum < f.LEdge+uint32(f.LEdgeNum); ledgeNum++ {
			e := bsp.Raw.LEdge[ledgeNum]
			if e == 0 {
				return nil, fmt.Errorf("ledge had value 0")
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
			tris = append(tris, Triangle{
				Face: f,
				A:    int(vs[0]),
				B:    int(vs[i+1]),
				C:    int(vs[i+2]),
			})
		}
	}
	return tris, nil
}
