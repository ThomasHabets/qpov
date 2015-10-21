package dist

import (
	"fmt"
	"regexp"
)

type Order struct {
	Package     string   // Path to tarfile on S3.
	Dir         string   // Dir to chdir to before rendering.
	File        string   // File in package to render.
	Destination string   // S3 directory to place results in.
	Args        []string // Povray args.
}

// S3Parse return bucket, dir, fn.
func S3Parse(s string) (string, string, string, error) {
	r := `^s3://([^/]+)/(?:(.*)/)?(.*)$`
	re := regexp.MustCompile(r)
	m := re.FindStringSubmatch(s)
	if len(m) != 4 {
		return "", "", "", fmt.Errorf("%q does not match %q", s, r)
	}
	return m[1], m[2], m[3], nil
}
