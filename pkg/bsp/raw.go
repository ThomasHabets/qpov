package bsp

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

// The file contains the raw file loading code.

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"log"
	"regexp"
	"strconv"

	"github.com/ThomasHabets/qpov/pkg/mdl"
)

const (
	// Sizes of various structs that are part of the file format.
	// This is to prevent accidentally adding fields to those structs.
	fileFaceSize    = 2 + 2 + 4 + 2 + 2 + 1 + 1 + 2 + 4
	fileTexInfoSize = 3*4 + 4 + 3*4 + 4 + 4 + 4
	fileModelSize   = 2*3*4 + 3*4 + 4*4 + 3*4
	fileMiptexSize  = 16 + 4 + 4 + 4*4
	fileVertexSize  = 4 * 3
	fileEdgeSize    = 2 + 2

	// BSP file version.
	Version = 29

	unusedMipTexOffset = uint32(4294967295)
)

var (
	coordRE        = regexp.MustCompile(`(-?[0-9.]+) (-?[0-9.]+) (-?[0-9.]+)`)
	entityKeyValRE = regexp.MustCompile(`^ *"([^"]+)" "([^"]+)"$`)
)

// A RawFace is a polygon as it appears in the BSP file.
type RawFace struct {
	PlaneID   uint16
	Side      uint16 // 0 if in front of the plane. This doesn't appear to be needed.
	LEdge     uint32 // First LEdge (see "RawLEdge" for more info).
	LEdgeNum  uint16 // Number of LEdges.
	TexinfoID uint16 // Texture information.

	// 0 = normal light map.
	// 1 = fast pulse.
	// 2 = slow pulse.
	// 3-10 = other light effects.
	// 0xff = no light map
	LightType uint8

	LightBase uint8 // 0xff = dark, 0 = bright.
	Light     [2]uint8
	Lightmap  uint32 // File offset
}

// A RawMipTex is the metadata about a texture.
// Textures are stored four times. One in original size, and three precalculated
// downsamples.
// Offsets are relative to where the current MipTex structure, not to beginning of
// file, beginning of texture area, or anything else sane. The textures are probably
// right after the miptex metadata (meaning Offset1 is 40), but it's not guaranteed.
type RawMipTex struct {
	NameBytes [16]byte // Name of the texture.
	Width     uint32   // Width of picture, must be a multiple of 8
	Height    uint32   // Height of picture, must be a multiple of 8
	Offset1   uint32   // Offset to full scale texture.
	Offset2   uint32   // Offset to 1/2 scale texture.
	Offset4   uint32   // Offset to 1/4 scale texture.
	Offset8   uint32   // Offset to 1/8 scale texture.
}

// Name returns the string representation of the ASCIIZ name.
// This is not put in RawMipTex because the struct must be fixed size.
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

// A RawEdge is the edge of one or more polygons in the file.
// From and To are indices in the vertex table.
// Edges are not referenced directly from polygons, only via LEdges.
type RawEdge struct {
	From uint16
	To   uint16
}

// A RawTexInfo is information about how to apply a texture (MipTex) onto a polygon.
// Texture coordinates are not attached to vertices directly and interpolated in 2D space,
// but are instead calculated by mapping world 3D coordinates onto the polygon plane.
//
// The polygon plane is defined here in the TexInfo by the two vectors VectorS and VectorT,
// which is strange since the RawFace already points to the plane definition that would
// reconstruct the vectors. DistS and DistT is used to align the texture (slide it around in the plane).
//
// A vertex's texture coordinates can be obtained with:
//   s = (v dot VectorS) + distS
//   t = (v dot VectorT) + distT
//
// I guess it's more space efficient to calculate s&t as needed from the texinfo rather than
// to store it with the LEdges, and that's why.
type RawTexInfo struct {
	VectorS   Vertex  // S vector, horizontal in texture space.
	DistS     float32 // Horizontal offset in texture space
	VectorT   Vertex  // T vector, vertical in texture space.
	DistT     float32 // Vertical offset in texture space
	TextureID uint32  // Index of Mip Texture must be in [0,numtex[
	Animated  uint32  // 0 for ordinary textures, 1 for water, etc.
}

type dentry struct {
	Offset uint32
	Size   uint32
}

// RawHeader is the first thing in the file.
type RawHeader struct {
	Version   uint32 // 29 (const Version)
	Entities  dentry // Entities (lights, start points, weapons, enemies...)
	Planes    dentry
	Miptex    dentry // Textures.
	Vertices  dentry
	Visilist  dentry // PVS.
	Nodes     dentry // BSP nodes.
	TexInfo   dentry // How to apply a miptex to a face.
	Faces     dentry // Polygons.
	Lightmaps dentry
	Clipnodes dentry
	Leaves    dentry // BSP leaves.
	Lface     dentry // List of faces. Used for BSP.
	Edges     dentry
	LEdges    dentry
	Models    dentry // See RawModel comment.
}

// A RawModel is the model definition of a some polygons.
// Most of level is in model 0. Others are doors and other movables.
//
// Models from BSP files show up in game as entities with model name "*N", where
// N is the index into this .bsp table.
type RawModel struct {
	BoundBoxMin, BoundBoxMax Vertex // The bounding box of the Model.
	Origin                   Vertex // Origin of model, usually (0,0,0).
	NodeID0                  uint32 // Index of first BSP node.
	NodeID1                  uint32 // Index of the first Clip node.
	NodeID2                  uint32 // Index of the second Clip node.
	NodeID3                  uint32 // Usually zero.
	NumLeafs                 uint32 // Number of BSP leaves.
	FaceID                   uint32 // Index of Faces
	FaceNum                  uint32 // Number of faces.
}

// Raw is the raw BSP file data.
//
// Well, it's slightly parsed, such as textures being turned into image.Image objects.
// But indirections such as Face->LEdge->Edge->Vertex are not removed.
type Raw struct {
	Header     RawHeader
	Vertex     []Vertex
	Face       []RawFace     // Polygons.
	MipTex     []RawMipTex   // Texture metadata.
	MipTexData []image.Image // Textures.
	Entities   []Entity      // Player start point, weapons, enemies, ...
	Edge       []RawEdge     // Connections between vertices.
	LEdge      []int32       // Connect faces with edges.
	TexInfo    []RawTexInfo  // How to apply a miptex to a face.
	Models     []RawModel    // Parts of geometry. For levels 0 is everything non-movable.
}

type myReader interface {
	io.Reader
	io.Seeker
}

func parseFloat32(s string) (float32, error) {
	t, err := strconv.ParseFloat(s, 64)
	return float32(t), err
}

func parseVertex(s string) (Vertex, error) {
	m := coordRE.FindStringSubmatch(s)
	if len(m) != 4 {
		return Vertex{}, fmt.Errorf("vertex coord parse fail: %q", s)
	}
	v := Vertex{}

	var err error
	if v.X, err = parseFloat32(m[1]); err != nil {
	} else if v.Y, err = parseFloat32(m[2]); err != nil {
	} else if v.Z, err = parseFloat32(m[3]); err != nil {
	}
	if err != nil {
		return Vertex{}, fmt.Errorf("vertex coord parse fail: %q", s)
	}
	return v, nil
}

// Entities is a big string in the file with a list of key values per entity.
// E.g.:
//   {
//     "classname" "light"
//     "origin" "1 2 3"
//   }
//   {
//     "classname" "weapon_shotgun"
//     "origin" "4 5 6"
//   }
func parseEntities(in string) ([]Entity, error) {
	buf := bytes.NewBuffer([]byte(in))
	scanner := bufio.NewScanner(buf)
	var ents []Entity
	for scanner.Scan() {
		if scanner.Text() == "\x00" {
			// TODO: why is there a null byte at the end?
			break
		}
		if scanner.Text() != "{" {
			return nil, fmt.Errorf("parse error, expected '{', got %q", scanner.Text())
		}
		ent := Entity{
			EntityID: len(ents),
			Data:     make(map[string]string),
		}
		for {
			if !scanner.Scan() {
				return nil, fmt.Errorf("unexpected EOF or error: %v", scanner.Err())
			}
			if scanner.Text() == "}" {
				break
			}
			m := entityKeyValRE.FindStringSubmatch(scanner.Text())
			if len(m) != 3 {
				return nil, fmt.Errorf("parse error on %q", scanner.Text())
			}
			ent.Data[m[1]] = m[2]
			switch m[1] {
			case "origin":
				var err error
				ent.Pos, err = parseVertex(m[2])
				if err != nil {
					return nil, fmt.Errorf("parsing origin: %v", err)
				}
			case "angle":
				a, err := strconv.ParseFloat(m[2], 64)
				if err != nil {
					return nil, fmt.Errorf("bad angle string: %q", m[2])
				}
				ent.Angle.Z = float32(a)
			}

		}
		if Verbose && (ent.Data["classname"] == "monster_ogre") {
			log.Printf("Entity %d is %v", len(ents), ent)
		}
		ents = append(ents, ent)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ents, nil
}

// LoadRaw loads a BSP file, doing minimal parsing.
func LoadRaw(r myReader) (*Raw, error) {
	raw := &Raw{}

	// Load file header.
	if err := binary.Read(r, binary.LittleEndian, &raw.Header); err != nil {
		return nil, err
	}
	if raw.Header.Version != Version {
		return nil, fmt.Errorf("wrong version %d, only %d supported", raw.Header.Version, Version)
	}

	// Load vertices.
	{
		if raw.Header.Vertices.Size%fileVertexSize != 0 {
			return nil, fmt.Errorf("vertex sizes %v not divisable by %v", raw.Header.Vertices.Size, fileVertexSize)
		}
		numVertices := raw.Header.Vertices.Size / fileVertexSize
		raw.Vertex = make([]Vertex, numVertices, numVertices)
		if _, err := r.Seek(int64(raw.Header.Vertices.Offset), 0); err != nil {
			return nil, fmt.Errorf("seeking to vertices at %v: %v", raw.Header.Vertices.Offset, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.Vertex); err != nil {
			return nil, fmt.Errorf("reading vertices data: %v", err)
		}
	}

	// Load faces.
	{
		if raw.Header.Faces.Size%fileFaceSize != 0 {
			return nil, fmt.Errorf("face sizes %v not divisable by %v", raw.Header.Faces.Size, fileFaceSize)
		}
		numFaces := raw.Header.Faces.Size / fileFaceSize
		raw.Face = make([]RawFace, numFaces, numFaces)
		if _, err := r.Seek(int64(raw.Header.Faces.Offset), 0); err != nil {
			return nil, fmt.Errorf("seeking to faces at %v: %v", raw.Header.Faces.Offset, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.Face); err != nil {
			return nil, fmt.Errorf("reading faces data: %v", err)
		}
	}

	// Load edges.
	{
		if raw.Header.Edges.Size%fileEdgeSize != 0 {
			return nil, fmt.Errorf("edge sizes %v not divisable by %v", raw.Header.Edges.Size, fileEdgeSize)
		}
		numEdges := raw.Header.Edges.Size / fileEdgeSize
		raw.Edge = make([]RawEdge, numEdges, numEdges)
		if _, err := r.Seek(int64(raw.Header.Edges.Offset), 0); err != nil {
			return nil, fmt.Errorf("seeking to edges at %v: %v", raw.Header.Edges.Offset, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.Edge); err != nil {
			return nil, fmt.Errorf("reading edges data: %v", err)
		}
	}

	// Load ledges.
	{
		ledgeSize := uint32(4)
		if raw.Header.LEdges.Size%ledgeSize != 0 {
			return nil, fmt.Errorf("ledge sizes %v not divisable by %v", raw.Header.LEdges.Size, ledgeSize)
		}
		numLEdges := raw.Header.LEdges.Size / ledgeSize
		raw.LEdge = make([]int32, numLEdges, numLEdges)
		if _, err := r.Seek(int64(raw.Header.LEdges.Offset), 0); err != nil {
			return nil, fmt.Errorf("seeking to LEdges at %v: %v", raw.Header.LEdges.Offset, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.LEdge); err != nil {
			return nil, fmt.Errorf("reading LEdges data: %v", err)
		}
		//fmt.Printf("LEdges: %v\n", les)
	}

	// Load texinfo.
	{
		if raw.Header.TexInfo.Size%fileTexInfoSize != 0 {
			return nil, fmt.Errorf("texInfo size %v not divisible by %v", raw.Header.TexInfo.Size, fileTexInfoSize)
		}
		numTexInfos := raw.Header.TexInfo.Size / fileTexInfoSize
		raw.TexInfo = make([]RawTexInfo, numTexInfos, numTexInfos)
		if _, err := r.Seek(int64(raw.Header.TexInfo.Offset), 0); err != nil {
			return nil, fmt.Errorf("seeking to texinfo at %v: %v", raw.Header.TexInfo.Offset, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.TexInfo); err != nil {
			return nil, fmt.Errorf("reading texinfo data: %v", err)
		}
	}

	// Load models.
	{
		if raw.Header.Models.Size%fileModelSize != 0 {
			return nil, fmt.Errorf("models size %v not divisible by %v", raw.Header.Models.Size, fileModelSize)
		}
		numModels := raw.Header.Models.Size / fileModelSize
		raw.Models = make([]RawModel, numModels, numModels)
		if _, err := r.Seek(int64(raw.Header.Models.Offset), 0); err != nil {
			return nil, fmt.Errorf("seeking to models at %v: %v", raw.Header.Models.Offset, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.Models); err != nil {
			return nil, fmt.Errorf("reading models data: %v", err)
		}
	}

	// Load miptex.
	{
		if _, err := r.Seek(int64(raw.Header.Miptex.Offset), 0); err != nil {
			return nil, fmt.Errorf("seeking to miptex at %v: %v", raw.Header.Miptex.Offset, err)
		}
		var numMipTex uint32
		if err := binary.Read(r, binary.LittleEndian, &numMipTex); err != nil {
			return nil, fmt.Errorf("reading miptex count: %v", err)
		}
		mipTexOfs := make([]uint32, numMipTex, numMipTex)
		raw.MipTex = make([]RawMipTex, numMipTex, numMipTex)
		if err := binary.Read(r, binary.LittleEndian, &mipTexOfs); err != nil {
			return nil, fmt.Errorf("reading %v miptex offsets: %v", numMipTex, err)
		}
		for n := range mipTexOfs {
			// Read header.
			if _, err := r.Seek(int64(raw.Header.Miptex.Offset+mipTexOfs[n]), 0); err != nil {
				return nil, fmt.Errorf("seeking to miptex %v header at %v+%v=%v: %v",
					n, raw.Header.Miptex.Offset, mipTexOfs[n], raw.Header.Miptex.Offset+mipTexOfs[n], err)
			}
			if mipTexOfs[n] == unusedMipTexOffset {
				// fmt.Printf("Skipping texture near %d\n", n)
				// Make fake texture that won't be referenced.
				raw.MipTexData = append(raw.MipTexData, image.NewPaletted(image.Rectangle{Max: image.Point{X: 8, Y: 8}}, mdl.QuakePalette))
				continue
			}
			if err := binary.Read(r, binary.LittleEndian, &raw.MipTex[n]); err != nil {
				return nil, fmt.Errorf("reading miptex %d header: %v", n, err)
			}

			// Read data.
			size := raw.MipTex[n].Width * raw.MipTex[n].Height
			data := make([]byte, size, size)
			pos := int64(raw.Header.Miptex.Offset + mipTexOfs[n] + raw.MipTex[n].Offset1)
			if _, err := r.Seek(pos, 0); err != nil {
				return nil, fmt.Errorf("seeking to miptex %d data at %v+%v+%v=%v: %v",
					n, raw.Header.Miptex.Offset, mipTexOfs[n], raw.MipTex[n].Offset1, pos, err)
			}
			if _, err := r.Read(data); err != nil {
				return nil, fmt.Errorf("reading %v bytes of miptex %v (%q) data at %v: %v", len(data), n, raw.MipTex[n].Name(), pos, err)
			}
			img := image.NewPaletted(image.Rectangle{
				Max: image.Point{X: int(raw.MipTex[n].Width), Y: int(raw.MipTex[n].Height)},
			}, mdl.QuakePalette)
			for bn, b := range data {
				img.SetColorIndex(bn%int(raw.MipTex[n].Width), bn/int(raw.MipTex[n].Width), b)
			}
			raw.MipTexData = append(raw.MipTexData, img)
		}
	}

	// Load entities.
	{
		if _, err := r.Seek(int64(raw.Header.Entities.Offset), 0); err != nil {
			return nil, fmt.Errorf("seeking to entities data at %v: %v", raw.Header.Entities.Offset, err)
		}
		entBytes := make([]byte, raw.Header.Entities.Size)
		if n, err := r.Read(entBytes); err != nil {
			return nil, fmt.Errorf("reading %v bytes of entities data: %v", raw.Header.Entities.Size, err)
		} else if uint32(n) != raw.Header.Entities.Size {
			return nil, fmt.Errorf("short read for entities: %d < %d", n, raw.Header.Entities.Size)
		}
		var err error
		raw.Entities, err = parseEntities(string(entBytes))
		if err != nil {
			return nil, fmt.Errorf("parsing entities: %v", err)
		}
	}
	return raw, nil
}
