package dist

type Order struct {
	Package     string   // Path to tarfile on S3.
	Dir         string   // Dir to chdir to before rendering.
	File        string   // File in package to render.
	Destination string   // S3 directory to place results in.
	Args        []string // Povray args.
}
