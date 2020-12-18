package dist

import (
	"testing"
)

func TestS3Parse(t *testing.T) {
	for _, test := range []struct {
		in                string
		bucket, dir, file string
		err               bool
	}{
		{"", "", "", "", true},
		{"s3://qpov", "", "", "", true},
		{"s3://qpov/", "qpov", "", "", false},
		{"s3://qpov/foo.pov", "qpov", "", "foo.pov", false},
		{"s3://qpov/foo/bar.pov", "qpov", "foo", "bar.pov", false},
		{"s3://qpov/foo.pov/", "qpov", "foo.pov", "", false},
	} {
		bucket, dir, file, err := S3Parse(test.in)
		if test.err != (err != nil) {
			t.Errorf("For %q want err %v, got %v", test.in, test.err, err)
		}
		if bucket != test.bucket {
			t.Errorf("For %q want bucket %q, got %q", test.in, test.bucket, bucket)
		}
		if dir != test.dir {
			t.Errorf("For %q want dir %q, got %q", test.in, test.dir, dir)
		}
		if file != test.file {
			t.Errorf("For %q want file %q, got %q", test.in, test.file, file)
		}
	}
}
