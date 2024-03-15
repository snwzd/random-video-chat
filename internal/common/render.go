package common

import (
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
)

type Template struct {
	tmpl *template.Template
}

func NewTemplate(parse string) (*Template, error) {
	parsedTmpl, err := template.ParseGlob(parse)
	if err != nil {
		return nil, err
	}

	return &Template{
		tmpl: parsedTmpl,
	}, nil
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}
