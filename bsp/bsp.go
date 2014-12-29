package bsp

// https://developer.valvesoftware.com/wiki/Source_BSP_File_Format

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
	Texinfo   dentry
	Faces     dentry
	Lightmaps dentry
	Clipnodes dentry
	Leaves    dentry
	Lface     dentry
	Edges     dentry
	Ledges    dentry
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
	Vertex []Vertex
}

type BSP struct {
	StartPos Vertex
	Polygons []Polygon
}

type myReader interface {
	io.Reader
	io.Seeker
}

func parseEntities(in string) ([]map[string]string, error) {
	buf := bytes.NewBuffer([]byte(in))
	scanner := bufio.NewScanner(buf)
	re := regexp.MustCompile(`^ *"([^"]+)" "([^"]+)"$`)
	var ents []map[string]string
	for scanner.Scan() {
		if scanner.Text() != "{" {
			break
			return nil, fmt.Errorf("parse error, expected '{', got %q", scanner.Text())
		}
		ent := make(map[string]string)
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
			if m[1] == "classname" {
				fmt.Printf("Class: %q\n", m[2])
			}
			ent[m[1]] = m[2]
		}
		ents = append(ents, ent)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading standard input:", err)
	}
	return ents, nil
}

func findStart(es []map[string]string) Vertex {
	for _, e := range es {
		if e["classname"] == "info_player_start" {
			return parseVertex(e["origin"])
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
	var h fileHeader
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if h.Version != Version {
		return nil, fmt.Errorf("wrong version %d, only %d supported", h.Version, Version)
	}
	ret := &BSP{}

	// Load vertices.
	numVertices := h.Vertices.Size / fileVertexSize
	vs := make([]fileVertex, numVertices, numVertices)
	if _, err := r.Seek(int64(h.Vertices.Offset), 0); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &vs); err != nil {
		return nil, err
	}

	// Load faces.
	if h.Faces.Size%fileFaceSize != 0 {
		return nil, fmt.Errorf("face sizes %v not divisable by %v", h.Faces.Size, fileFaceSize)
	}
	numFaces := h.Faces.Size / fileFaceSize
	fs := make([]fileFace, numFaces, numFaces)
	if _, err := r.Seek(int64(h.Faces.Offset), 0); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &fs); err != nil {
		return nil, err
	}

	// Load edges.
	numEdges := h.Edges.Size / fileEdgeSize
	if h.Edges.Size%fileEdgeSize != 0 {
		return nil, fmt.Errorf("edge sizes %v not divisable by %v", h.Edges.Size, fileEdgeSize)
	}
	es := make([]fileEdge, numEdges, numEdges)
	if _, err := r.Seek(int64(h.Edges.Offset), 0); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &es); err != nil {
		return nil, err
	}

	// Load entities.
	if _, err := r.Seek(int64(h.Entities.Offset), 0); err != nil {
		return nil, err
	}
	entBytes := make([]byte, h.Entities.Size)
	if n, err := r.Read(entBytes); err != nil {
		return nil, err
	} else if uint32(n) != h.Entities.Size {
		return nil, fmt.Errorf("short read: %d < %d", n, h.Entities.Size)
	}
	ents, err := parseEntities(string(entBytes))
	if err != nil {
		return nil, err
	}
	ret.StartPos = findStart(ents)

	for _, f := range fs {
		var p Polygon
		first, last := f.LEdge, f.LEdge+uint32(f.LEdgeNum)
		fmt.Printf("Edges: %v (%d to %d of %v)\n", f.LEdgeNum, first, last-1, numEdges)
		for i := first; i < last; i++ {
			//fmt.Printf(" Edge: %v of %v\n", i, numEdges)
			if i >= numEdges {
				continue
			}
			//fmt.Printf("  Vertex: %v -> %v (of %v)\n", es[i].From, es[i].To, numVertices)
			v0 := Vertex{
				X: vs[es[i].From].X,
				Y: vs[es[i].From].Y,
				Z: vs[es[i].From].Z,
			}
			//fmt.Printf("   Coord: %v\n", v0)
			//fmt.Printf("   Coord: %v\n", v1)
			p.Vertex = append(p.Vertex, v0)
		}
		if last < numEdges {
			p.Vertex = append(p.Vertex, Vertex{
				X: vs[es[last-1].To].X,
				Y: vs[es[last-1].To].Y,
				Z: vs[es[last-1].To].Z,
			})
		}
		if len(p.Vertex) > 0 {
			ret.Polygons = append(ret.Polygons, p)
		}
	}
	return ret, nil
}
