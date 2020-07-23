package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"llts/js"
	"llts/js/ast"
	"llts/js/selector"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

var supportedExts = map[string]bool{
	"ts":  true,
	"tsx": true,
}

type file struct {
	path    string
	content string
}

func (f file) ext() string {
	ext := filepath.Ext(f.path)
	if strings.HasPrefix(ext, ".") {
		ext = ext[1:]
	}
	return ext
}

func parseJS(ctx context.Context, f file, dialect js.Dialect) string {
	l := new(js.Lexer)
	p := new(js.Parser)
	l.Init(f.content)
	l.Dialect = dialect
	result := "ok"
	errHandler := func(se js.SyntaxError) bool { return false }
	p.Init(errHandler, func(nt js.NodeType, offset, endoffset int) {})
	if err := p.Parse(ctx, l); err != nil {
		result = "parse_err"
		var suffix string
		if err, ok := err.(js.SyntaxError); ok {
			suffix = fmt.Sprintf(" on `%v`", f.content[err.Offset:err.Endoffset])
		}
		fmt.Printf("%v: %v%v\n", f.path, err, suffix)
	}

	return result
}

func (f file) tryParse(ctx context.Context) string {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("%v: recovered: %v\n", f.path, r)
		}
	}()

	switch f.ext() {
	case "ts":
		return parseJS(ctx, f, js.Typescript)
	case "tsx":
		return parseJS(ctx, f, js.TypescriptJsx)
	}

	return "no_parser"
}

func preloadAll(root string) ([]file, error) {
	var ret []file
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			if info.Name() == ".git" {
				fmt.Println("skipping .git")
				return filepath.SkipDir
			}
			return nil
		}

		if ext := strings.TrimPrefix(filepath.Ext(path), "."); !supportedExts[ext] {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(data) {
			fmt.Printf("skipping a non-utf8 file: %v\n", path)
			return nil
		}
		ret = append(ret, file{path, string(data)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func parse(ctx context.Context, files []file) {
	results := map[string]int{}
	for _, f := range files {
		outcome := f.tryParse(ctx)
		results[outcome+" ("+f.ext()+")"]++
	}

	var keys []string
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Println()
	for _, k := range keys {
		fmt.Printf("%10v %v\n", results[k], k)
	}

	for _, f := range files {
		outcome := f.tryParse(ctx)
		results[outcome+" ("+f.ext()+")"]++
	}
}

func main() {
	ctx := context.Background()
	files, err := preloadAll(os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("loaded %v files\n", len(files))

	//try to parse files
	parse(ctx, files)

	for _, file := range files {
		tree, err := ast.Parse(ctx, file.path, string(file.content), js.StopOnFirstError)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(tree.Root().Type())
		for _, n := range tree.Root().Children(selector.Any) {
			fmt.Println(n.Type())
			fmt.Println(n.Child(selector.Any).Type())
		}

	}

}
