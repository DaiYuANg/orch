package dsl

import (
	"fmt"
	"github.com/expr-lang/expr"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/wkhere/bcl"
	"gopkg.in/yaml.v3"
)

type Parser struct {
}

func (p *Parser) ParseBcl(body []byte) error {
	var config Config
	err := bcl.Unmarshal(body, config)
	if err != nil {
		return err
	}
	_, _ = hclsyntax.ParseExpression(body, "", hcl.Pos{Line: 1, Column: 1})
	return nil
}

func (p *Parser) parse(body []byte) error {
	var config Config
	env := map[string]interface{}{
		"greet":   "Hello, %v!",
		"names":   []string{"world", "you"},
		"sprintf": fmt.Sprintf,
	}

	code := `sprintf(greet, names[0])`

	_, err := expr.Compile(code, expr.Env(env))
	err = yaml.Unmarshal(body, &config)
	if err != nil {
		return err
	}
	return nil
}
