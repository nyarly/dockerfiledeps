package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/builder/dockerfile/parser"
)

type depRecord struct {
	from     reference.Reference
	addpaths []string
}

var records = map[string]depRecord{}

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %s <build root directory>", os.Args[0])
	}

	registryURL := os.Args[1]
	dir := os.Args[2]

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.Name() == "Dockerfile" {
			r, err := filepath.Rel(dir, path)
			if err != nil {
				log.Fatal(err)
			}
			imageName := filepath.Dir(r) // strip './'

			addDockerfile(imageName, path)
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("# file dependencies based on ADD and COPY directives")
	for name, record := range records {
		if record.isLocal() {
			fmt.Printf(".build/%s: .build/%s\n\n", name, record.localFrom(registryURL))
		} else {
			remote := record.remoteFrom()

			filename := strings.Replace(remote, "_", "_u_", -1) // ick, but that's how it goes
			filename = strings.Replace(filename, "/", "_s_", -1)
			filename = strings.Replace(filename, ":", "_c_", -1)
			filename = filename + ".image"

			fmt.Printf(".build/%s: .remote/%s\n\n", name, filename)
			fmt.Printf(".remote/%[1]s: .pull-once | .remote\n"+
				"\tdocker pull %[2]s\n"+
				"\t@docker image inspect %[2]s --format '{{.Id}}' > $(TMPDIR)/%[1]s\n"+
				"\t@diff $(TMPDIR)/%[1]s $@ \\\n\t  || mv $(TMPDIR)/%[1]s $@\n\n", filename, remote)
		}

		fmt.Printf("build-all: build-%s\n\n", name)
		fmt.Printf("push-all: push-%s\n\n", name) // leaves?
		fmt.Printf(".build/%s: | .build\n\n", name)
		fmt.Printf(".build/%s: %s\n\n", name, strings.Join(record.filedeps(dir, name), " "))
	}
}

func (r depRecord) isLocal() bool {
	if t, is := r.from.(reference.Tagged); is && t.Tag() == "local" {
		return true
	}
	return false
}

func (r depRecord) localFrom(url string) string {
	if n, is := r.from.(reference.Named); is {
		if strings.Index(n.Name(), url) != 0 {
			log.Fatalf("FROM %s uses a registry URL != %q", n.String(), url)
			return n.Name()[len(url):]
		}
	}
	log.Fatalf("FROM %s doesn't parse as a named Docker reference", r.from.String())
	return ""
}

func (r depRecord) remoteFrom() string {
	return r.from.String()
}

func (r depRecord) filedeps(base, name string) []string {
	ds := []string{}

	for _, a := range r.addpaths {
		filepath.Walk(filepath.Join(base, name, a), func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				d, err := filepath.Rel(base, path)
				if err != nil {
					log.Fatal(err)
				}
				ds = append(ds, d)
			}
			return nil
		})
	}

	return ds
}

func addDockerfile(name, path string) {
	ast, err := parseDockerfile(path)
	if err != nil {
		log.Fatal(err)
	}
	depRec := extractDeps(ast)
	records[name] = depRec
}

func parseDocker(f io.Reader) (*parser.Node, error) {
	d := parser.Directive{LookingForDirectives: true}
	parser.SetEscapeToken(parser.DefaultEscapeToken, &d)

	return parser.Parse(f, &d)
}

func parseDockerfile(path string) (*parser.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseDocker(f)
}

var chownRE = regexp.MustCompile(`^--chown`)
var urlRE = regexp.MustCompile(`^\S+://`)

func extractDeps(ast *parser.Node) depRecord {
	record := depRecord{}
	// Parse for ENV SOUS_RUN_IMAGE_SPEC
	// Parse for FROM
	for _, node := range ast.Children {
		switch node.Value {
		case "from":
			var err error
			record.from, err = reference.Parse(node.Next.Value)
			if err != nil {
				log.Fatal(err)
			}
		case "add", "copy":
			path := node.Next.Value
			if chownRE.MatchString(path) {
				path = node.Next.Next.Value
			}
			if !urlRE.MatchString(path) {
				record.addpaths = append(record.addpaths, path)
			}
		}
	}
	return record
}
