package untrustedflowsource

import (
	"errors"
	"fmt"

	"github.com/gagliardetto/codemill/x"
	. "github.com/gagliardetto/utilz"
)

const (
	Kind x.ModelKind = "UntrustedFlowSource"
)

type Handler struct{}

const (
	MethodSelf = "Self"
)

//
func (han *Handler) ScavengeMethods() []*x.XMethod {
	return []*x.XMethod{
		{
			Name:      MethodSelf,
			IsSelf:    true,
			Selectors: []*x.XSelector{},
		},
	}
}
func (han *Handler) Validate(mdl *x.XModel) error {
	// TODO
	Sfln(
		"%s: Validating model %q",
		Kind,
		mdl.Name,
	)
	if len(mdl.Methods) != 1 {
		return fmt.Errorf("wrong number of methods; expected 1, got %v", len(mdl.Methods))
	}
	if !mdl.Methods[0].IsSelf {
		return errors.New("First method is not self")
	}
	if mdl.Methods[0].Name != MethodSelf {
		return fmt.Errorf("First method is not called %s", MethodSelf)
	}
	return nil
}
