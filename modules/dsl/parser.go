package dsl

import (
	"fmt"
	"github.com/expr-lang/expr"
	"gopkg.in/yaml.v3"
)

func parse(body []byte) error {
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
