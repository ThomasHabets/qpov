// Package bsp loads Quake BSP files.
//
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
// References:
// * https://developer.valvesoftware.com/wiki/Source_BSP_File_Format
// * http://www.gamers.org/dEngine/quake/spec/quake-spec34/qkspec_4.htm
// * http://www.gamers.org/dEngine/quake/QDP/qmapspec.html
package bsp

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// Prefix for all BSP file macros.
	macroPrefix = "modelprefix_"
)

var (
	// POV output parameters to add to all light_sources.
	// Values found by experimentation.
	// Test values that have worked out:
	//  Package        Demo        Level    Light values     Gamma
	//  base install   demo1.dem   e1m3     3, 20, 3         2.0
	lightMultiplier   = flag.Float64("light_multiplier", 3.0, "Light strength multiplier.")
	lightFadeDistance = flag.Float64("light_fade_distance", 20.0, "Light fade distance.")
	lightFadePower    = flag.Float64("light_fade_power", 2.0, "Light fade power.")

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

// Load loads a BSP model (map) from something that reads and seeks.
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

// ModelMacroPrefix returns the model macro prefix for a given model filename.
func ModelMacroPrefix(s string) string {
	re := regexp.MustCompile(`[/.*-]`)
	return macroPrefix + re.ReplaceAllString(s, "_")
}

// POVLights returns the static light sources in a BSP, in POV-Ray format.
func (bsp *BSP) POVLights() string {
	ret := []string{}
	for _, ent := range bsp.Raw.Entities {
		// TODO: There are more complicated light sources.
		if strings.HasPrefix(ent.Data["classname"], "light") {
			brightness, err := strconv.ParseFloat(ent.Data["light"], 64)
			if err != nil {
				brightness = 200.0
			}
			brightness /= 200.0 // 200.0 is Quake baseline.
			// TODO: I think brightness should actually multiply with fade_distance, not color.
			ret = append(ret, fmt.Sprintf(`
light_source {
  <%v>
  rgb<1,1,1>*%g*%g
  fade_distance %g
  fade_power %g
}`, ent.Pos.String(), brightness, *lightMultiplier, *lightFadeDistance, *lightFadePower))
		}
	}
	return strings.Join(ret, "\n")
}

// remapVertex returns an existing vertex ID if it's in the list,
// else add it to the list and return that ID.
// This is used to prevent duplicate vertices when creating triangles for the BSP.
func (bsp *BSP) remapVertex(in int, list *[]Vertex, vertexMap map[int]int) int {
	if newV, found := vertexMap[in]; found {
		return newV
	}
	newV := len(*list)
	*list = append(*list, bsp.Raw.Vertex[in])
	vertexMap[in] = newV
	return newV
}

// overrideTexture changes given textures to something more POV-Ray-y.
// If texture should be overriden, return the texture data and true.
// If just a normal texture, second return is false.
func (bsp *BSP) overrideTexture(miptex uint32) (string, bool) {
	mipName := bsp.Raw.MipTex[miptex].Name()
	switch mipName {
	case "*lava1":
		return `
      normal { bumps 0.08 scale <1,0.25,0.35>*1 turbulence 0.6 }
      pigment { rgbf<0.5,0.0,0,0.2> }
      finish {
        reflection { 0.1 }
        diffuse 0.55
      }
`, true
	case "*04water1", "*04water2", "*slime0": // Green-brown slime.
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
	if strings.HasPrefix(mipName, "*") {
		return `
      normal { bumps 0.08 scale <1,0.25,0.35>*1 turbulence 0.6 }
      pigment { rgbf<0,0,1,0.2> }
      finish {
        reflection 0.3
        diffuse 0.55
      }
`, true
	}

	return "", false
}

// POVTriangleMesh returns the triangle mesh of the BSP as macros starting with the prefix given.
// One BSP can contain multiple models.
// If withTextures is false, everything will be flatshaded with the flatColor.
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
				triangles[n].a = bsp.remapVertex(triangles[n].a, &localVertices, vertexMap)
				triangles[n].b = bsp.remapVertex(triangles[n].b, &localVertices, vertexMap)
				triangles[n].c = bsp.remapVertex(triangles[n].c, &localVertices, vertexMap)

				oldMipTex := bsp.Raw.TexInfo[triangles[n].face.TexinfoID].TextureID
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
				ti := bsp.Raw.TexInfo[tri.face.TexinfoID]
				mip := bsp.Raw.MipTex[ti.TextureID]

				v0 := localVertices[tri.a]
				v1 := localVertices[tri.b]
				v2 := localVertices[tri.c]

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
				textureID := bsp.Raw.TexInfo[tri.face.TexinfoID].TextureID
				tris = append(tris, fmt.Sprintf("<%d,%d,%d>,%d", tri.a, tri.b, tri.c, mipTexMap[textureID]))
			}
			ret += fmt.Sprintf("  face_indices { %d, %s }\n", len(tris), strings.Join(tris, ","))
		}

		// TODO: Add normal indices.

		// Add texture coord indices.
		if withTextures {
			var tris []string
			for n, tri := range triangles {
				if false {
					tris = append(tris, fmt.Sprintf("<%v,%v,%v>", tri.a, tri.b, tri.c))
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

type triangle struct {
	face    RawFace
	a, b, c int // Triangle vertex index.
}

// makeTriangles takes the faces from one model in the BSP and returns them as triangles.
// The reason for using triangles instead of polygons is that:
// 1) They can never go wrong in terms of geometry.
// 2) Triangles is what POV-Ray mesh2 operator wants.
func (bsp *BSP) makeTriangles(modelNumber int) ([]triangle, error) {
	skipFace := make(map[int]bool)
	for nm, m := range bsp.Raw.Models {
		for n := int(m.FaceID); n < int(m.FaceID+m.FaceNum); n++ {
			if nm != modelNumber {
				skipFace[n] = true
			}
		}
	}

	tris := []triangle{}
	for fn, f := range bsp.Raw.Face {
		if skipFace[fn] {
			continue
		}
		texName := bsp.Raw.MipTex[bsp.Raw.TexInfo[f.TexinfoID].TextureID].Name()
		switch texName {
		case "trigger": // Don't draw triggers.
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
			tris = append(tris, triangle{
				face: f,
				a:    int(vs[0]),
				b:    int(vs[i+1]),
				c:    int(vs[i+2]),
			})
		}
	}
	return tris, nil
}
