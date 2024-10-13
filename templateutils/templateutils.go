package templateutils

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/unionj-cloud/toolkit/caller"
)

// String return result of calling template Execute as string
func String(tmplname, tmpl string, data interface{}) (string, error) {
	var (
		sqlBuf bytes.Buffer
		err    error
		tpl    *template.Template
	)
	funcs := map[string]any{
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
	}
	tpl = template.Must(template.New(tmplname).Funcs(funcs).Parse(tmpl))
	if err = tpl.Execute(&sqlBuf, data); err != nil {
		return "", errors.Wrap(err, caller.NewCaller().String())
	}
	return strings.TrimSpace(sqlBuf.String()), nil
}

// StringBlock return result of calling template Execute as string
func StringBlock(tmplname, tmpl string, block string, data interface{}) (string, error) {
	var (
		sqlBuf bytes.Buffer
		err    error
		tpl    *template.Template
	)
	tpl = template.Must(template.New(tmplname).Parse(tmpl))
	if err = tpl.ExecuteTemplate(&sqlBuf, block, data); err != nil {
		return "", errors.Wrap(err, caller.NewCaller().String())
	}
	return strings.TrimSpace(sqlBuf.String()), nil
}

// StringBlockMysql return result of calling template Execute as string from template file
func StringBlockMysql(tmpl string, block string, data interface{}) (string, error) {
	var (
		sqlBuf  bytes.Buffer
		err     error
		tpl     *template.Template
		funcMap map[string]interface{}
	)
	tpl = template.New(filepath.Base(tmpl))
	funcMap = make(map[string]interface{})
	funcMap["FormatTime"] = formatTime
	funcMap["BoolToInt"] = boolToInt
	funcMap["Eval"] = Eval(tpl)
	funcMap["TrimSuffix"] = trimSuffix
	funcMap["isNil"] = func(t interface{}) bool {
		return t == nil
	}
	tpl = template.Must(tpl.Funcs(funcMap).ParseFiles(tmpl))
	if err = tpl.ExecuteTemplate(&sqlBuf, block, data); err != nil {
		return "", errors.Wrap(err, caller.NewCaller().String())
	}
	return strings.TrimSpace(sqlBuf.String()), nil
}

// BlockMysql return result of calling template Execute as string from template file
func BlockMysql(tmplname, tmpl string, block string, data interface{}) (string, error) {
	var (
		sqlBuf  bytes.Buffer
		err     error
		tpl     *template.Template
		funcMap map[string]interface{}
	)
	tpl = template.New(tmplname)
	funcMap = make(map[string]interface{})
	funcMap["FormatTime"] = formatTime
	funcMap["BoolToInt"] = boolToInt
	funcMap["Eval"] = Eval(tpl)
	funcMap["TrimSuffix"] = trimSuffix
	funcMap["isNil"] = func(t interface{}) bool {
		return t == nil
	}
	tpl = template.Must(tpl.Funcs(funcMap).Parse(tmpl))
	if err = tpl.ExecuteTemplate(&sqlBuf, block, data); err != nil {
		return "", errors.Wrap(err, caller.NewCaller().String())
	}
	return strings.TrimSpace(sqlBuf.String()), nil
}
