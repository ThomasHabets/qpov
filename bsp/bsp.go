package bsp

import (
	"encoding/binary"
	"fmt"
	"io"
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

const fileVertexSize = 4*3
type fileVertex struct {
	X,Y,Z float32
}

const fileEdgeSize = 2 + 2
type fileEdge struct {
	From uint16
	To uint16
}

type Vertex struct {
	X,Y,Z float32
}

type Polygon struct {
	Vertex []Vertex
}

type BSP struct {
	Polygons []Polygon
}

type myReader interface {
	io.Reader
	io.Seeker
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
	if h.Faces.Size % fileFaceSize != 0 {
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
	if h.Edges.Size % fileEdgeSize != 0 {
		return nil, fmt.Errorf("edge sizes %v not divisable by %v", h.Edges.Size, fileEdgeSize)
	}
	es := make([]fileEdge, numEdges, numEdges)
	if _, err := r.Seek(int64(h.Edges.Offset), 0); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &es); err != nil {
		return nil, err
	}

	for _, f := range fs {
		var p Polygon
		first, last := f.LEdge, f.LEdge + uint32(f.LEdgeNum)
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
		if len(p.Vertex) > 0{
			ret.Polygons = append(ret.Polygons, p)
		}
	}
	return ret, nil
}
