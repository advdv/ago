package initwizard

type Wizard struct {
	builder FormBuilder
	runner  FormRunner
}

func New(builder FormBuilder, runner FormRunner) *Wizard {
	return &Wizard{
		builder: builder,
		runner:  runner,
	}
}

func (w *Wizard) Run(defaultIdent string) (Result, error) {
	var result Result
	form := w.builder.Build(defaultIdent, &result)

	if err := w.runner.Run(form); err != nil {
		return Result{}, err
	}

	return result, nil
}
