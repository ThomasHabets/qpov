package bsp

// https://developer.valvesoftware.com/wiki/Source_BSP_File_Format
// http://www.gamers.org/dEngine/quake/spec/quake-spec34/qkspec_4.htm

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
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

type fileHeader struct {
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

const fileFaceSize = 2 + 2 + 4 + 2 + 2 + 1 + 1 + 2 + 4

type fileFace struct {
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

const fileTexInfoSize = 3*4 + 4 + 3*4 + 4 + 4 + 4

type fileTexInfo struct {
	SectorS   Vertex  // S vector, horizontal in texture space)
	DistS     float32 // horizontal offset in texture space
	VectorT   Vertex  // T vector, vertical in texture space
	DistT     float32 // vertical offset in texture space
	TextureID uint32  // Index of Mip Texture must be in [0,numtex[
	Animated  uint32  // 0 for ordinary textures, 1 for water
}

const fileMiptexSize = 16 + 4 + 4 + 4*4

type fileMiptex struct {
	NameBytes [16]byte // Name of the texture.
	Width     uint32   // width of picture, must be a multiple of 8
	Height    uint32   // height of picture, must be a multiple of 8
	Offset1   uint32   // offset to u_char Pix[width   * height]
	Offset2   uint32   // offset to u_char Pix[width/2 * height/2]
	Offset4   uint32   // offset to u_char Pix[width/4 * height/4]
	Offset8   uint32   // offset to u_char Pix[width/8 * height/8]
}

func (f *fileMiptex) Name() string {
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

type fileVertex struct {
	X, Y, Z float32
}

const fileEdgeSize = 2 + 2

type fileEdge struct {
	From uint16
	To   uint16
}

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
	StartPos Vertex
	Polygons []Polygon
	Entities []Entity
}

type myReader interface {
	io.Reader
	io.Seeker
}

type Entity struct {
	EntityID int
	Data     map[string]string
	Pos      Vertex
	Angle    Vertex
	Frame    uint8
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

func findStart(es []Entity) Vertex {
	for _, e := range es {
		if e.Data["classname"] == "info_player_start" {
			return parseVertex(e.Data["origin"])
		}
	}
	log.Fatal("can't find start")
	panic("hello")
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

func Load(r myReader) (*BSP, error) {
	// Load file header.
	var h fileHeader
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if h.Version != Version {
		return nil, fmt.Errorf("wrong version %d, only %d supported", h.Version, Version)
	}
	ret := &BSP{}

	// Load vertices.
	var vs []fileVertex
	{
		if h.Vertices.Size%fileVertexSize != 0 {
			return nil, fmt.Errorf("vertex sizes %v not divisable by %v", h.Vertices.Size, fileVertexSize)
		}
		numVertices := h.Vertices.Size / fileVertexSize
		vs = make([]fileVertex, numVertices, numVertices)
		if _, err := r.Seek(int64(h.Vertices.Offset), 0); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &vs); err != nil {
			return nil, err
		}
	}

	// Load faces.
	var fs []fileFace
	{
		if h.Faces.Size%fileFaceSize != 0 {
			return nil, fmt.Errorf("face sizes %v not divisable by %v", h.Faces.Size, fileFaceSize)
		}
		numFaces := h.Faces.Size / fileFaceSize
		fs = make([]fileFace, numFaces, numFaces)
		if _, err := r.Seek(int64(h.Faces.Offset), 0); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &fs); err != nil {
			return nil, err
		}
	}

	// Load edges.
	var es []fileEdge
	{
		if h.Edges.Size%fileEdgeSize != 0 {
			return nil, fmt.Errorf("edge sizes %v not divisable by %v", h.Edges.Size, fileEdgeSize)
		}
		numEdges := h.Edges.Size / fileEdgeSize
		es = make([]fileEdge, numEdges, numEdges)
		if _, err := r.Seek(int64(h.Edges.Offset), 0); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &es); err != nil {
			return nil, err
		}
	}

	// Load ledges.
	var les []int32
	{
		ledgeSize := uint32(4)
		if h.LEdges.Size%ledgeSize != 0 {
			return nil, fmt.Errorf("ledge sizes %v not divisable by %v", h.LEdges.Size, ledgeSize)
		}
		numLEdges := h.LEdges.Size / ledgeSize
		les = make([]int32, numLEdges, numLEdges)
		if _, err := r.Seek(int64(h.LEdges.Offset), 0); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &les); err != nil {
			return nil, err
		}
		//fmt.Printf("LEdges: %v\n", les)
	}

	// Load texinfo.
	var tes []fileTexInfo
	{
		if h.TexInfo.Size%fileTexInfoSize != 0 {
			return nil, fmt.Errorf("texInfo size %v not divisible by %v", h.TexInfo.Size, fileTexInfoSize)
		}
		numTexInfos := h.TexInfo.Size / fileTexInfoSize
		tes = make([]fileTexInfo, numTexInfos, numTexInfos)
		if _, err := r.Seek(int64(h.TexInfo.Offset), 0); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &tes); err != nil {
			return nil, err
		}
	}

	// Load miptex.
	var mtes []fileMiptex
	{
		if _, err := r.Seek(int64(h.Miptex.Offset), 0); err != nil {
			return nil, err
		}
		var numMipTex uint32
		if err := binary.Read(r, binary.LittleEndian, &numMipTex); err != nil {
			return nil, err
		}
		log.Printf("Number of textures: %d", numMipTex)
		mipTexOfs := make([]uint32, numMipTex, numMipTex)
		mtes = make([]fileMiptex, numMipTex, numMipTex)
		if err := binary.Read(r, binary.LittleEndian, &mipTexOfs); err != nil {
			return nil, err
		}
		for n := range mipTexOfs {
			if _, err := r.Seek(int64(h.Miptex.Offset+mipTexOfs[n]), 0); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &mtes[n]); err != nil {
				return nil, err
			}
			if Verbose {
				log.Printf("Miptex %s: %v", mtes[n].Name(), mtes[n])
			}
		}
	}

	// Load entities.
	{
		if _, err := r.Seek(int64(h.Entities.Offset), 0); err != nil {
			return nil, err
		}
		entBytes := make([]byte, h.Entities.Size)
		if n, err := r.Read(entBytes); err != nil {
			return nil, err
		} else if uint32(n) != h.Entities.Size {
			return nil, fmt.Errorf("short read: %d < %d", n, h.Entities.Size)
		}
		var err error
		ret.Entities, err = parseEntities(string(entBytes))
		if err != nil {
			return nil, err
		}
		ret.StartPos = findStart(ret.Entities)
	}

	// Assemble polygons.
	for faceIndex, f := range fs {
		p := Polygon{
			Texture: mtes[tes[f.TexinfoID].TextureID].Name(),
		}
		first, last := f.LEdge, f.LEdge+uint32(f.LEdgeNum)
		if Verbose {
			log.Printf("LEdges: %v (%d to %d of %v)\n", f.LEdgeNum, first, last-1, len(les))
		}
		for i := first; i < last; i++ {
			if Verbose {
				log.Printf(" LEdge: %v\n", i)
			}
			if i >= uint32(len(les)) {
				log.Fatalf("Index to LEdge OOB")
			}
			e := les[i]
			if Verbose {
				log.Printf("  Edge %d\n", e)
			}
			if e == 0 {
				log.Fatalf("Tried to reference edge 0")
			}
			var vi0, vi1 uint16
			if e < 0 {
				e = -e
				vi1, vi0 = es[e].From, es[e].To
			} else {
				vi0, vi1 = es[e].From, es[e].To
			}
			//fmt.Printf("  Vertex: %v -> %v (of %v)\n", es[e].From, es[e].To, numVertices)
			v0 := Vertex{
				X: vs[vi0].X,
				Y: vs[vi0].Y,
				Z: vs[vi0].Z,
			}
			v1 := Vertex{
				X: vs[vi1].X,
				Y: vs[vi1].Y,
				Z: vs[vi1].Z,
			}
			if Verbose {
				log.Printf("   Edge coord: %v -> %v\n", v0, v1)
			}
			if i == first {
				p.Vertex = append(p.Vertex, v0)
			}
			p.Vertex = append(p.Vertex, v1)
		}
		if f.Side != 0 {
			for n := range p.Vertex {
				n2 := len(p.Vertex) - n - 1
				p.Vertex[n], p.Vertex[n2] = p.Vertex[n2], p.Vertex[n]
			}
		}
		if true {
			blah := false
			if faceIndex == 3942 {
				blah = true
			}
			if blah {
				log.Printf("Face %v (%v) became triangle %v", faceIndex, p, len(ret.Polygons))
			}
			for i := 0; i < len(p.Vertex)-2; i++ {
				ret.Polygons = append(ret.Polygons, Polygon{
					Texture: p.Texture,
					Vertex: []Vertex{
						Vertex{
							X: p.Vertex[0].X,
							Y: p.Vertex[0].Y,
							Z: p.Vertex[0].Z,
						},
						Vertex{
							X: p.Vertex[i+1].X,
							Y: p.Vertex[i+1].Y,
							Z: p.Vertex[i+1].Z,
						},
						Vertex{
							X: p.Vertex[i+2].X,
							Y: p.Vertex[i+2].Y,
							Z: p.Vertex[i+2].Z,
						},
						Vertex{
							X: p.Vertex[0].X,
							Y: p.Vertex[0].Y,
							Z: p.Vertex[0].Z,
						},
					},
				})
				if blah {
					log.Printf("  Triangle: %v", ret.Polygons[len(ret.Polygons)-1])
				}
			}
		} else {
			ret.Polygons = append(ret.Polygons, p)
			if Verbose {
				log.Printf("Added:  %v\n", p)
			}
		}
	}
	return ret, nil
}
