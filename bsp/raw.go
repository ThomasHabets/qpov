package bsp

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

	"github.com/ThomasHabets/bsparse/mdl"
)

type RawFace struct {
	PlaneID   uint16
	Side      uint16
	LEdge     uint32 // First edge.
	LEdgeNum  uint16 // Number of edges.
	TexinfoID uint16
	TypeLight uint8
	BaseLight uint8
	Light     [2]uint8
	Lightmap  uint32
}

type RawMipTex struct {
	NameBytes [16]byte // Name of the texture.
	Width     uint32   // width of picture, must be a multiple of 8
	Height    uint32   // height of picture, must be a multiple of 8
	Offset1   uint32   // offset to u_char Pix[width   * height]
	Offset2   uint32   // offset to u_char Pix[width/2 * height/2]
	Offset4   uint32   // offset to u_char Pix[width/4 * height/4]
	Offset8   uint32   // offset to u_char Pix[width/8 * height/8]
}

type RawEdge struct {
	From uint16
	To   uint16
}

type RawTexInfo struct {
	VectorS   Vertex  // S vector, horizontal in texture space)
	DistS     float32 // horizontal offset in texture space
	VectorT   Vertex  // T vector, vertical in texture space
	DistT     float32 // vertical offset in texture space
	TextureID uint32  // Index of Mip Texture must be in [0,numtex[
	Animated  uint32  // 0 for ordinary textures, 1 for water
}

type RawHeader struct {
	Version   uint32
	Entities  dentry
	Planes    dentry
	Miptex    dentry
	Vertices  dentry
	Visilist  dentry
	Nodes     dentry
	TexInfo   dentry
	Faces     dentry
	Lightmaps dentry
	Clipnodes dentry
	Leaves    dentry
	Lface     dentry
	Edges     dentry
	LEdges    dentry
	Models    dentry
}

type RawModel struct {
	BoundBoxMin, BoundBoxMax Vertex // The bounding box of the Model
	Origin                   Vertex // origin of model, usually (0,0,0)
	NodeID0                  uint32 // index of first BSP node
	NodeID1                  uint32 // index of the first Clip node
	NodeID2                  uint32 // index of the second Clip node
	NodeID3                  uint32 // usually zero
	NumLeafs                 uint32 // number of BSP leaves
	FaceID                   uint32 // index of Faces
	FaceNum                  uint32 // Number of faces.
}

type Raw struct {
	Header     RawHeader
	Vertex     []Vertex
	Face       []RawFace
	MipTex     []RawMipTex
	MipTexData []image.Image
	Entities   []Entity
	Edge       []RawEdge
	LEdge      []int32
	TexInfo    []RawTexInfo
	Models     []RawModel
}

type myReader interface {
	io.Reader
	io.Seeker
}

func parseVertex(s string) Vertex {
	re := regexp.MustCompile(`(-?[0-9.]+) (-?[0-9.]+) (-?[0-9.]+)`)
	m := re.FindStringSubmatch(s)
	if len(m) != 4 {
		log.Fatalf("Not a vertex: %q", s)
	}
	v := Vertex{}
	t, err := strconv.ParseFloat(m[1], 64)
	v.X = float32(t)
	t, err = strconv.ParseFloat(m[2], 64)
	v.Y = float32(t)
	t, err = strconv.ParseFloat(m[3], 64)
	v.Z = float32(t)
	if err != nil {
		log.Fatalf("Not a vertex: %q", s)
	}
	return v
}

func parseEntities(in string) ([]Entity, error) {
	buf := bytes.NewBuffer([]byte(in))
	scanner := bufio.NewScanner(buf)
	re := regexp.MustCompile(`^ *"([^"]+)" "([^"]+)"$`)
	var ents []Entity
	for scanner.Scan() {
		if scanner.Text() == "\x00" {
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
				return nil, fmt.Errorf("EOF or error")
			}
			if scanner.Text() == "}" {
				break
			}
			m := re.FindStringSubmatch(scanner.Text())
			if len(m) != 3 {
				return nil, fmt.Errorf("parse error on %q", scanner.Text())
			}
			ent.Data[m[1]] = m[2]
			switch m[1] {
			case "origin":
				ent.Pos = parseVertex(m[2])
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
		return nil, fmt.Errorf("reading standard input:", err)
	}
	return ents, nil
}

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
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.Vertex); err != nil {
			return nil, err
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
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.Face); err != nil {
			return nil, err
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
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.Edge); err != nil {
			return nil, err
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
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.LEdge); err != nil {
			return nil, err
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
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.TexInfo); err != nil {
			return nil, err
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
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &raw.Models); err != nil {
			return nil, err
		}
	}

	// Load miptex.
	{
		if _, err := r.Seek(int64(raw.Header.Miptex.Offset), 0); err != nil {
			return nil, err
		}
		var numMipTex uint32
		if err := binary.Read(r, binary.LittleEndian, &numMipTex); err != nil {
			return nil, err
		}
		mipTexOfs := make([]uint32, numMipTex, numMipTex)
		raw.MipTex = make([]RawMipTex, numMipTex, numMipTex)
		if err := binary.Read(r, binary.LittleEndian, &mipTexOfs); err != nil {
			return nil, err
		}
		for n := range mipTexOfs {
			// Read header.
			if _, err := r.Seek(int64(raw.Header.Miptex.Offset+mipTexOfs[n]), 0); err != nil {
				return nil, err
			}
			if mipTexOfs[n] == 4294967295 {
				continue
			}
			if err := binary.Read(r, binary.LittleEndian, &raw.MipTex[n]); err != nil {
				return nil, err
			}

			// Read data.
			size := raw.MipTex[n].Width * raw.MipTex[n].Height
			data := make([]byte, size, size)
			pos := int64(raw.Header.Miptex.Offset + mipTexOfs[n] + raw.MipTex[n].Offset1)
			if _, err := r.Seek(pos, 0); err != nil {
				return nil, fmt.Errorf("seeking to miptex data: %v", err)
			}
			if _, err := r.Read(data); err != nil {
				return nil, fmt.Errorf("reading %v bytes of miptex %q data at %v: %v", len(data), raw.MipTex[n].Name(), pos, err)
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
			return nil, err
		}
		entBytes := make([]byte, raw.Header.Entities.Size)
		if n, err := r.Read(entBytes); err != nil {
			return nil, err
		} else if uint32(n) != raw.Header.Entities.Size {
			return nil, fmt.Errorf("short read: %d < %d", n, raw.Header.Entities.Size)
		}
		var err error
		raw.Entities, err = parseEntities(string(entBytes))
		if err != nil {
			return nil, err
		}
	}

	return raw, nil
}
