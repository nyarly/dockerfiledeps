package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/builder/dockerfile/parser"
)

type depRecord struct {
	from     string
	addpaths []string
}

var records = map[string]depRecord{}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <build root directory>", os.Args[0])
	}

	dir := os.Args[1]

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

	fmt.Printf(".build: \n\tmkdir -p .build\n\n")
	fmt.Println("# file dependencies based on ADD and COPY directives")
	for name, record := range records {
		fmt.Printf(".build/%s: | .build\n", name)
		fmt.Printf(".build/%s: %s\n\n", name, strings.Join(record.filedeps(dir, name), " "))
	}
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
			record.from = node.Next.Value
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
