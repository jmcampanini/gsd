package cli

import "github.com/jmcampanini/gsd/internal/render"

// commandSpec is the data by which the parallel task, project, and area
// command constructors differ: the usage line, both help texts, and the
// mutation verb used in human output.
type commandSpec struct {
	long  string
	short string
	use   string
	verb  render.MutationVerb
}
