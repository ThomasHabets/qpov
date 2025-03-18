package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/golang/protobuf/proto"

	pb "github.com/ThomasHabets/qpov/pkg/dist/qpov"
)

func main() {
	flag.Parse()

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fz, err := gzip.NewReader(f)
	if err != nil {
		log.Fatal(err)
	}

	b, err := ioutil.ReadAll(fz)
	if err != nil {
		log.Fatal(err)
	}

	var p pb.RenderingMetadata
	if err := proto.Unmarshal(b, &p); err != nil {
		log.Fatal(err)
	}
	fmt.Println(proto.MarshalTextString(&p))
}
