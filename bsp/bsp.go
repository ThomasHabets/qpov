// Package bsp loads Quake BSP files.
//
// References:
// * https://developer.valvesoftware.com/wiki/Source_BSP_File_Format
// * http://www.gamers.org/dEngine/quake/spec/quake-spec34/qkspec_4.htm
// * http://www.gamers.org/dEngine/quake/QDP/qmapspec.html
package bsp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// POV output parameters to add to all light_sources.
	// Values found by experimentation.
	lightMultiplier   = 0.7   // Multiplier to light intensity.
	lightFadeDistance = 120.0 // fade_distance
	lightFadePower    = 2.0   // fade_power
)

var (
	Verbose = false
)

type Vertex struct {
	X, Y, Z float32
}

func (v *Vertex) String() string {
	return fmt.Sprintf("%g,%g,%g", v.X, v.Y, v.Z)
}

func (v *Vertex) DotProduct(w Vertex) float64 {
	return float64(v.X*w.X + v.Y*w.Y + v.Z*w.Z)
}

func (v *Vertex) Sub(w Vertex) *Vertex {
	return &Vertex{
		X: v.X - w.X,
		Y: v.Y - w.Y,
		Z: v.Z - w.Z,
	}
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
			ret = append(ret, fmt.Sprintf(`
light_source {
  <%v>
  rgb<1,1,1>*%g*%g
  fade_distance %g
  fade_power %g
}`, ent.Pos.String(), brightness, lightMultiplier, lightFadeDistance, lightFadePower))
		}
	}
	return strings.Join(ret, "\n")
}

func (bsp *BSP) remapVertex(in int, list *[]Vertex, vertexMap map[int]int) int {
	if newV, found := vertexMap[in]; found {
		return newV
	}
	newV := len(*list)
	*list = append(*list, bsp.Raw.Vertex[in])
	vertexMap[in] = newV
	return newV
}

func (bsp *BSP) overrideTexture(miptex uint32) (string, bool) {
	mipName := bsp.Raw.MipTex[miptex].Name()
	switch mipName {
	case "*lava1":
		return `
      normal { bumps 0.08 scale <1,0.25,0.35>*1 turbulence 0.6 }
      pigment { rgbf<1,0.0,0,0.2> }
      finish {
        reflection { 0.1 }
        diffuse 0.55
      }
`, true
	case "*04water1", "*slime0": // Green-brown slime.
		return `
      normal { bumps 0.08 scale <1,0.25,0.35>*1 turbulence 0.6 }
      pigment { rgbf<6/256,74/256,0,0.2> }
      finish {
        reflection { 0.1 }
      }
`, true
	case "*water", "*water0":
		return `
      normal { bumps 0.08 scale <1,0.25,0.35>*1 turbulence 0.6 }
      pigment { rgbf<0,0,1,0.2> }
      finish {
        reflection 0.3
        diffuse 0.55
      }
`, true
	case "*teleport":
		return "", false
	}

	// Default animator.
	if mipName[0] == '*' {
		return `
      normal { bumps 0.08 scale <1,0.25,0.35>*1 turbulence 0.6 }
      pigment { rgbf<1,0,0,0.2> }
      finish {
        reflection 0.3
        diffuse 0.55
      }
`, true
	}

	return "", false
}

func (bsp *BSP) POVTriangleMesh(prefix string, withTextures bool, flatColor string) (string, error) {
	var ret string
	for modelNumber := range bsp.Raw.Models {
		ret += fmt.Sprintf("#macro %s_%d(pos,rot,textureprefix)\n", prefix, modelNumber)

		triangles, err := bsp.makeTriangles(modelNumber)
		if err != nil {
			return "", nil
		}
		if len(triangles) == 0 {
			ret += "#end\n"
			continue
		}

		// Find used vertices and textures.
		// Vertice indices are simply changed and localVertices will be looked at later on.
		// For faces, because of the indirections via texinfo, both mipTexMap and localMipTex are
		// needed later on.
		// TODO: Change this to work the same way, for consistency?
		var localVertices []Vertex           // Local vertices
		mipTexMap := make(map[uint32]uint32) // Map from global texture ID to local.
		var localMipTex []uint32             // Map from local texture Id to global.
		{
			vertexMap := make(map[int]int) // Map from global vertex ID to local. Only used to avoid dups.
			for n := range triangles {
				triangles[n].A = bsp.remapVertex(triangles[n].A, &localVertices, vertexMap)
				triangles[n].B = bsp.remapVertex(triangles[n].B, &localVertices, vertexMap)
				triangles[n].C = bsp.remapVertex(triangles[n].C, &localVertices, vertexMap)

				oldMipTex := bsp.Raw.TexInfo[triangles[n].Face.TexinfoID].TextureID
				_, found := mipTexMap[oldMipTex]
				if !found {
					newMipTex := uint32(len(mipTexMap))
					mipTexMap[oldMipTex] = newMipTex
					localMipTex = append(localMipTex, oldMipTex)
				}
			}
		}

		ret += "object { mesh2 {\n"
		// Add vertices.
		{
			vs := []string{}
			for _, v := range localVertices {
				vs = append(vs, fmt.Sprintf("<%s>", v.String()))
			}
			ret += fmt.Sprintf("  vertex_vectors { %d, %s }\n", len(localVertices), strings.Join(vs, ","))
		}

		// Add texture coordinates.
		if withTextures {
			vs := []string{}
			for _, tri := range triangles {
				ti := bsp.Raw.TexInfo[tri.Face.TexinfoID]
				mip := bsp.Raw.MipTex[ti.TextureID]

				v0 := localVertices[tri.A]
				v1 := localVertices[tri.B]
				v2 := localVertices[tri.C]

				a := ti.VectorS
				b := ti.VectorT
				texWidth, texHeight := float64(mip.Width), float64(mip.Height)
				vs = append(vs, fmt.Sprintf("<%v,%v>",
					(v0.DotProduct(a)+float64(ti.DistS))/texWidth,
					(v0.DotProduct(b)+float64(ti.DistT))/texHeight,
				))
				vs = append(vs, fmt.Sprintf("<%v,%v>",
					(v1.DotProduct(a)+float64(ti.DistS))/texWidth,
					(v1.DotProduct(b)+float64(ti.DistT))/texHeight,
				))
				vs = append(vs, fmt.Sprintf("<%v,%v>",
					(v2.DotProduct(a)+float64(ti.DistS))/texWidth,
					(v2.DotProduct(b)+float64(ti.DistT))/texHeight,
				))
			}
			ret += fmt.Sprintf("  uv_vectors { %d, %s }\n", len(vs), strings.Join(vs, ","))
		}

		// TODO: add normals.

		// Add textures.
		{
			var textures []string
			for _, n := range localMipTex {
				texture := fmt.Sprintf(`
      uv_mapping
      pigment {
        image_map {
          png concat(textureprefix, "/texture_%d.png")
          interpolate 2
        }
        rotate <180,0,0>
      }
      finish {
        reflection {0.03}
        diffuse 0.55
      }
`, n)
				if !withTextures {
					texture = fmt.Sprintf("pigment{%s}", flatColor)
				}
				if tex, do := bsp.overrideTexture(n); do {
					texture = tex
				}
				textures = append(textures, fmt.Sprintf("// %s (#%v)\n%s", bsp.Raw.MipTex[n].Name(), n, texture))
			}
			ret += fmt.Sprintf("texture_list { %d, texture {%s} }\n", len(textures), strings.Join(textures, "}\ntexture{\n"))
		}

		// Add faces.
		{
			var tris []string
			for _, tri := range triangles {
				textureID := bsp.Raw.TexInfo[tri.Face.TexinfoID].TextureID
				tris = append(tris, fmt.Sprintf("<%d,%d,%d>,%d", tri.A, tri.B, tri.C, mipTexMap[textureID]))
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
				} else if false {
					tris = append(tris, fmt.Sprintf("<%d,%d,%d>", n, n+1, n+2))
				} else {
					tris = append(tris, fmt.Sprintf("<%d,%d,%d>", n*3, n*3+1, n*3+2))
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
	A, B, C int // Triangle vertex index.
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
