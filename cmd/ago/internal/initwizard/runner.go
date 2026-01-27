package initwizard

import (
	"io"

	"github.com/charmbracelet/huh"
)

type FormRunner interface {
	Run(form *huh.Form) error
}

type InteractiveRunner struct{}

func NewInteractiveRunner() *InteractiveRunner {
	return &InteractiveRunner{}
}

func (r *InteractiveRunner) Run(form *huh.Form) error {
	return form.Run()
}

type AccessibleRunner struct {
	output io.Writer
	input  io.Reader
}

func NewAccessibleRunner(output io.Writer, input io.Reader) *AccessibleRunner {
	return &AccessibleRunner{
		output: output,
		input:  input,
	}
}

func (r *AccessibleRunner) Run(form *huh.Form) error {
	return form.
		WithAccessible(true).
		WithOutput(r.output).
		WithInput(r.input).
		Run()
}
