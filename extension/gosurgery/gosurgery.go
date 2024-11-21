package gosurgery

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/molon/genx"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// TODO: 由于 go/ast 的 comments 设计缺陷，几乎无法对代码进行手术，所以只用其来做解析，而不是修改。
// TODO: 本尝试使用 dst 来实现，但是后续发现 dst 也有其他缺陷并且并不积极维护了，所以也不作考虑。

type GoFile struct {
	*genx.File
	af   *ast.File
	fset *token.FileSet
	tf   *token.File
	cm   ast.CommentMap
}

func (f *GoFile) lineStartOffset(p token.Pos) int {
	return f.tf.Position(f.tf.LineStart(f.tf.Position(p).Line)).Offset
}

func (f *GoFile) offset(p token.Pos) int {
	return f.tf.Position(p).Offset
}

func convertToComment(code string) string {
	lines := strings.Split(code, "\n")

	var minIndent string
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		indentLen := len(line) - len(trimmed)
		if minIndent == "" || indentLen < len(minIndent) {
			minIndent = line[:indentLen]
		}
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if minIndent == "" {
			lines[i] = "// " + line
			continue
		}
		if strings.HasPrefix(line, minIndent) {
			lines[i] = minIndent + "// " + line[len(minIndent):]
		}
	}

	return strings.Join(lines, "\n")
}

func (f *GoFile) findDuplicates(declMap map[ast.Decl][]*Ident, userIdentMap map[string]*Ident) (genx.Replacements, error) {
	var rs genx.Replacements
	for decl, idents := range declMap {
		var identsShouldComment []*Ident
		for _, ident := range idents {
			if _, exists := userIdentMap[ident.Mark]; exists {
				identsShouldComment = append(identsShouldComment, ident)
			}
		}

		if len(identsShouldComment) == 0 {
			continue
		}

		if len(identsShouldComment) == len(idents) {
			declLineStart := f.lineStartOffset(decl.Pos())
			declEnd := f.offset(decl.End())
			rs = append(rs, &genx.Replacement{
				Start: declLineStart,
				End:   declEnd,
				Text:  convertToComment(f.File.Content[declLineStart:declEnd]),
			})
			continue
		}

		// type const var
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			return nil, errors.Errorf("failed to convert decl to gen decl %s", decl)
		}

		// type
		/*
			type (
				A int
				B string
			)
		*/
		if genDecl.Tok == token.TYPE {
			for _, ident := range identsShouldComment {
				spec, ok := ident.Obj.Decl.(*ast.TypeSpec)
				if !ok {
					return nil, errors.Errorf("failed to get type spec for ident %s", ident.Mark)
				}
				specLineStart := f.lineStartOffset(spec.Pos())
				specEnd := f.offset(spec.End())
				rs = append(rs, &genx.Replacement{
					Start: specLineStart,
					End:   specEnd,
					Text:  convertToComment(f.File.Content[specLineStart:specEnd]),
				})
			}
			continue
		}

		// const var
		if genDecl.Tok != token.CONST && genDecl.Tok != token.VAR {
			return nil, errors.Errorf("unsupported gen decl type %s", genDecl.Tok)
		}
		/*
			const (
				a,b,c = 1,2,3
				d = 4
			)
		*/
		specMap := map[*ast.ValueSpec][]*Ident{}
		for _, ident := range identsShouldComment {
			spec, ok := ident.Obj.Decl.(*ast.ValueSpec)
			if !ok {
				return nil, errors.Errorf("failed to get value spec for ident %s", ident.Mark)
			}
			specMap[spec] = append(specMap[spec], ident)
		}
		for spec, idents := range specMap {
			var identsShouldComment []*Ident
			for _, ident := range idents {
				if _, exists := userIdentMap[ident.Mark]; exists {
					identsShouldComment = append(identsShouldComment, ident)
				}
			}

			if len(identsShouldComment) == 0 {
				continue
			}

			if len(identsShouldComment) == len(spec.Names) {
				specLineStart := f.lineStartOffset(spec.Pos())
				specEnd := f.offset(spec.End())
				rs = append(rs, &genx.Replacement{
					Start: specLineStart,
					End:   specEnd,
					Text:  convertToComment(f.File.Content[specLineStart:specEnd]),
				})
				continue
			}

			for _, ident := range identsShouldComment {
				identStart := f.offset(ident.Pos())
				identEnd := f.offset(ident.End())
				_, index, ok := lo.FindIndexOf(spec.Names, func(v *ast.Ident) bool { return v.Name == ident.Name })
				if !ok {
					return nil, errors.Errorf("failed to find index of ident %s in value spec", ident.Name)
				}
				// TODO: 或许改成先移除，再在下面位置构造新的并且注释会更好，会涉及到 type 和 value 的构造，const 没 type
				// TODO: 为什么要这样改呢，因为 /* "*/" */ 这种情况会出错，没有单行注释来的直接
				shouldCommentValue := len(spec.Values) > 1
				if index == 0 {
					nextStart := f.offset(spec.Names[index+1].Pos())
					rs = append(rs, &genx.Replacement{
						Start: identStart,
						End:   nextStart,
						Text:  "/* " + strings.TrimSpace(f.File.Content[identStart:nextStart]) + " */",
					})
					if shouldCommentValue {
						start := f.offset(spec.Values[index].Pos())
						end := f.offset(spec.Values[index+1].Pos())
						rs = append(rs, &genx.Replacement{
							Start: start,
							End:   end,
							Text:  "/* " + strings.TrimSpace(f.File.Content[start:end]) + " */",
						})
					}
				} else {
					prevEnd := f.offset(spec.Names[index-1].End())
					rs = append(rs, &genx.Replacement{
						Start: prevEnd,
						End:   identEnd,
						Text:  "/* " + strings.TrimSpace(f.File.Content[prevEnd:identEnd]) + " */",
					})
					if shouldCommentValue {
						start := f.offset(spec.Values[index-1].End())
						end := f.offset(spec.Values[index].End())
						rs = append(rs, &genx.Replacement{
							Start: start,
							End:   end,
							Text:  "/* " + strings.TrimSpace(f.File.Content[start:end]) + " */",
						})
					}
				}
			}
		}
	}
	return rs, nil
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(string(s[0])) + s[1:]
}

func isLowerCase(c byte) bool {
	return c >= 'a' && c <= 'z'
}

func getHookMark(identMark string) string {
	if receiver, methodName, ok := strings.Cut(identMark, "."); ok {
		if len(methodName) > 0 && isLowerCase(methodName[0]) {
			return receiver + ".hook" + capitalize(methodName)
		}
		return receiver + ".Hook" + methodName
	}

	if len(identMark) > 0 && isLowerCase(identMark[0]) {
		return "hook" + capitalize(identMark)
	}
	return "Hook" + identMark
}

const hookReplacementPlaceholder = "___JUST_PLACEHOLDER_FOR_HOOK_REPLACEMENT___"

var reHookReplacement = regexp.MustCompile(`\{\s*return\s+` + hookReplacementPlaceholder + `\s*\}`)

func (f *GoFile) findHooks(declMap map[ast.Decl][]*Ident, userIdentMap map[string]*Ident) (genx.Replacements, error) {
	var rs genx.Replacements
	for decl, idents := range declMap {
		ident := idents[0]
		if _, exists := userIdentMap[ident.Mark]; exists {
			continue // should commented
		}

		d, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		hookMark := getHookMark(ident.Mark)
		if _, exists := userIdentMap[hookMark]; !exists {
			continue
		}

		bodySrc, err := f.formatNode(d.Body, true)
		if err != nil {
			return nil, errors.Wrap(err, "format funcDecl.Body")
		}

		var hookCallExpr ast.Expr
		if d.Recv != nil && len(d.Recv.List) > 0 {
			receiverName := d.Recv.List[0].Names[0].Name
			methodName := strings.SplitN(hookMark, ".", 2)[1]
			hookCallExpr = ast.NewIdent(fmt.Sprintf("%s.%s", receiverName, methodName))
		} else {
			hookCallExpr = ast.NewIdent(hookMark)
		}

		originalFuncLit := &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  d.Type.Params,
				Results: d.Type.Results,
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.ReturnStmt{
						Results: []ast.Expr{
							&ast.Ident{
								Name: hookReplacementPlaceholder,
							},
						},
					},
				},
			},
		}
		var args []ast.Expr
		for _, field := range d.Type.Params.List {
			for _, v := range field.Names {
				args = append(args, ast.NewIdent(v.Name))
			}
		}
		wrappedBody := &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CallExpr{
							Fun: &ast.CallExpr{
								Fun:  hookCallExpr,
								Args: []ast.Expr{originalFuncLit},
							},
							Args: args,
						},
					},
				},
			},
		}
		wrappedBodySrc, err := f.formatNode(wrappedBody, false)
		if err != nil {
			return nil, errors.Wrap(err, "format wrappedBody")
		}
		wrappedBodySrc = reHookReplacement.ReplaceAllString(wrappedBodySrc, bodySrc)
		formattedSrc, err := format.Source([]byte(wrappedBodySrc))
		if err != nil {
			return nil, errors.Wrap(err, "format.Source")
		}
		wrappedBodySrc = string(formattedSrc)

		rs = append(rs, &genx.Replacement{
			Start: f.offset(d.Body.Pos()),
			End:   f.offset(d.Body.End()),
			Text:  wrappedBodySrc,
		})
	}
	return rs, nil
}

func (f *GoFile) formatNode(node ast.Node, withComments bool) (string, error) {
	var comments []*ast.CommentGroup
	if withComments {
		comments = f.cm.Filter(node).Comments()
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, f.fset, &printer.CommentedNode{Node: node, Comments: comments}); err != nil {
		return "", errors.Wrap(err, "format node")
	}
	return buf.String(), nil
}

func (f *GoFile) refactorBy(ctx context.Context, userIdentMap map[string]*Ident) error {
	var replacements genx.Replacements

	identMap := buildIdentMap(f.af.Decls)
	declMap := map[ast.Decl][]*Ident{}
	for _, ident := range identMap {
		declMap[ident.Decl] = append(declMap[ident.Decl], ident)
	}

	{
		rs, err := f.findDuplicates(declMap, userIdentMap)
		if err != nil {
			return err
		}
		replacements = append(replacements, rs...)
	}

	{
		rs, err := f.findHooks(declMap, userIdentMap)
		if err != nil {
			return err
		}
		replacements = append(replacements, rs...)
	}

	return f.File.ApplyReplacements(ctx, replacements)
}

func getReceiverType(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
		// example: with generics
		// func (u *User[T, A]) List
		if expr, ok := e.X.(ast.Expr); ok {
			return getReceiverType(expr)
		}
	case *ast.IndexListExpr: // func (u User[T, A]) List
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return e.Name
	}
	return ""
}

type Ident struct {
	*ast.Ident
	Decl ast.Decl
	Mark string
}

func buildIdentMap(decls []ast.Decl) map[string]*Ident {
	identMap := make(map[string]*Ident)
	for _, decl := range decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			mark := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recvType := getReceiverType(d.Recv.List[0].Type)
				mark = recvType + "." + d.Name.Name
			}
			identMap[mark] = &Ident{Ident: d.Name, Decl: decl, Mark: mark}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					identMap[s.Name.Name] = &Ident{Ident: s.Name, Decl: decl, Mark: s.Name.Name}
				case *ast.ValueSpec:
					for _, ident := range s.Names {
						identMap[ident.Name] = &Ident{Ident: ident, Decl: decl, Mark: ident.Name}
					}
				}
			}
		}
	}
	return identMap
}

func parseGoFiles(files []*genx.File) ([]*GoFile, error) {
	var goFiles []*GoFile
	for _, f := range files {
		fset := token.NewFileSet()
		af, err := parser.ParseFile(fset, "", f.Content, parser.ParseComments)
		if err != nil {
			return nil, errors.Wrapf(err, "parse file %s", f.RelPath)
		}
		goFiles = append(goFiles, &GoFile{
			File: f,
			af:   af,
			fset: fset,
			tf:   fset.File(af.Pos()),
			cm:   ast.NewCommentMap(fset, af, af.Comments),
		})
	}
	return goFiles, nil
}

func Surgery(ctx context.Context, generatedFiles []*genx.File, userFiles []*genx.File) error {
	filter := func(file *genx.File, _ int) bool { return filepath.Ext(file.RelPath) == ".go" }
	generatedFiles = lo.Filter(generatedFiles, filter)
	userFiles = lo.Filter(userFiles, filter)
	if len(generatedFiles) == 0 || len(userFiles) == 0 {
		return nil
	}

	userGoFiles, err := parseGoFiles(userFiles)
	if err != nil {
		return err
	}
	var userDecls []ast.Decl
	for _, uf := range userGoFiles {
		userDecls = append(userDecls, uf.af.Decls...)
	}
	userIdentMap := buildIdentMap(userDecls)

	generatedGoFiles, err := parseGoFiles(generatedFiles)
	if err != nil {
		return err
	}
	for _, gf := range generatedGoFiles {
		if err := gf.refactorBy(ctx, userIdentMap); err != nil {
			return errors.Wrapf(err, "surgery file %s", gf.File.RelPath)
		}
	}
	return nil
}
